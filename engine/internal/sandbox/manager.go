package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

type Manager struct {
	dockerClient *client.Client
	sandboxImage string
}

func NewManager(ctx context.Context, sandboxImage string) *Manager {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return &Manager{sandboxImage: sandboxImage}
	}
	if _, err := dockerClient.Ping(ctx); err != nil {
		return &Manager{sandboxImage: sandboxImage}
	}
	return &Manager{dockerClient: dockerClient, sandboxImage: sandboxImage}
}

func (m *Manager) Health(ctx context.Context) map[string]any {
	status := map[string]any{"healthy": false, "mode": "local", "sandbox_image": m.sandboxImage}
	if m.dockerClient == nil {
		status["message"] = "docker daemon unavailable, using local fallback"
		return status
	}
	ping, err := m.dockerClient.Ping(ctx)
	if err != nil {
		status["message"] = err.Error()
		return status
	}
	status["healthy"] = true
	status["mode"] = "docker-ready"
	status["api_version"] = ping.APIVersion
	return status
}

func (m *Manager) PrepareWorkspace(ctx context.Context, experiment models.Experiment, task models.Task, artifacts []models.ArtifactVersion) (string, map[string]string, error) {
	workspace, err := os.MkdirTemp("", "frameval-workspace-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp workspace: %w", err)
	}
	switch experiment.WorkspaceSourceType {
	case "local_path":
		if strings.TrimSpace(experiment.LocalPath) == "" {
			return "", nil, fmt.Errorf("local workspace source requires local_path")
		}
		if err := copyDir(experiment.LocalPath, workspace); err != nil {
			return "", nil, fmt.Errorf("copy local workspace: %w", err)
		}
	case "git_url":
		if strings.TrimSpace(experiment.GitURL) == "" {
			return "", nil, fmt.Errorf("git workspace source requires git_url")
		}
		if err := os.RemoveAll(workspace); err != nil {
			return "", nil, fmt.Errorf("reset temp workspace for git clone: %w", err)
		}
		if err := cloneGitWorkspace(ctx, experiment.GitURL, experiment.GitRef, workspace); err != nil {
			return "", nil, err
		}
	case "empty":
		// Intentionally start from a blank workspace for greenfield tasks.
	default:
		// Honour the task-defined workspace recipe when the experiment defers to it.
		switch task.WorkspaceMode {
		case "git":
			if strings.TrimSpace(task.WorkspaceGitURL) == "" {
				return "", nil, fmt.Errorf("task %s declares git workspace but workspace_git_url is empty", task.ID)
			}
			if err := os.RemoveAll(workspace); err != nil {
				return "", nil, fmt.Errorf("reset temp workspace for task git clone: %w", err)
			}
			if err := cloneGitWorkspace(ctx, task.WorkspaceGitURL, task.WorkspaceGitRef, workspace); err != nil {
				return "", nil, err
			}
		case "empty":
			// Greenfield task, intentionally blank.
		default:
			if task.CodebasePath != "" {
				if err := copyDir(task.CodebasePath, workspace); err != nil {
					return "", nil, fmt.Errorf("copy task codebase: %w", err)
				}
			}
		}
	}
	// Tasks no longer ship with per-task tests/ folders — each task encodes its
	// verification inside its `test_cases[*].test_command` (e.g. `go build ./...`).
	for _, artifact := range artifacts {
		if artifact.FilePath == "" {
			continue
		}
		targetPath := filepath.Join(workspace, artifact.FilePath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", nil, fmt.Errorf("create artifact directory: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(artifact.Content), 0o644); err != nil {
			return "", nil, fmt.Errorf("write artifact file: %w", err)
		}
	}
	if strings.TrimSpace(task.SetupScript) != "" {
		if _, err := m.RunShell(ctx, workspace, nil, task.SetupScript); err != nil {
			return "", nil, fmt.Errorf("run setup script: %w", err)
		}
	}
	snapshot, err := snapshotFiles(workspace)
	if err != nil {
		return "", nil, err
	}
	return workspace, snapshot, nil
}

func (m *Manager) RunShell(ctx context.Context, workspace string, env map[string]string, script string) (string, error) {
	if m.dockerClient != nil {
		return m.runInDocker(ctx, workspace, env, script)
	}
	return m.runLocalShell(ctx, workspace, env, script)
}

func (m *Manager) runLocalShell(ctx context.Context, workspace string, env map[string]string, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Dir = workspace
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (m *Manager) runInDocker(ctx context.Context, workspace string, env map[string]string, script string) (string, error) {
	resp, err := m.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image:      m.sandboxImage,
			WorkingDir: "/workspace",
			Cmd:        []string{"sh", "-c", script},
			Env:        buildSandboxEnv(env),
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		nil,
		"frameval-sandbox-"+uuid.NewString(),
	)
	if err != nil {
		return "", fmt.Errorf("create sandbox container: %w", err)
	}
	containerID := resp.ID
	defer func() {
		_ = m.dockerClient.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()
	if err := m.copyWorkspaceToContainer(ctx, containerID, workspace); err != nil {
		return "", err
	}
	if err := m.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start sandbox container: %w", err)
	}
	statusCh, errCh := m.dockerClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case waitErr := <-errCh:
		if waitErr != nil {
			return "", fmt.Errorf("wait for sandbox container: %w", waitErr)
		}
	case <-statusCh:
	}
	output, logErr := m.readContainerLogs(ctx, containerID)
	if err := m.copyWorkspaceFromContainer(ctx, containerID, workspace); err != nil {
		return output, err
	}
	inspect, err := m.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		if logErr != nil {
			return output, logErr
		}
		return output, nil
	}
	if inspect.State != nil && inspect.State.ExitCode != 0 {
		if logErr != nil {
			return output, logErr
		}
		return output, fmt.Errorf("sandbox command exited with code %d", inspect.State.ExitCode)
	}
	return output, logErr
}

func (m *Manager) CollectOutputFiles(workspace string) ([]models.OutputFile, error) {
	files := make([]models.OutputFile, 0)
	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".venv" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		files = append(files, models.OutputFile{Path: filepath.ToSlash(rel), Content: string(content)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect output files: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func (m *Manager) DiffSnapshots(before map[string]string, files []models.OutputFile) string {
	var lines []string
	after := map[string]string{}
	for _, file := range files {
		hash := sha256.Sum256([]byte(file.Content))
		after[file.Path] = fmt.Sprintf("%x", hash[:])
	}
	for path, hash := range after {
		if before[path] == "" {
			lines = append(lines, "+ "+path)
			continue
		}
		if before[path] != hash {
			lines = append(lines, "~ "+path)
		}
	}
	for path := range before {
		if after[path] == "" {
			lines = append(lines, "- "+path)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func copyDir(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src string, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Close()
}

func snapshotFiles(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".venv" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(content)
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = fmt.Sprintf("%x", hash[:])
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot workspace: %w", err)
	}
	return result, nil
}

func cloneGitWorkspace(ctx context.Context, gitURL string, gitRef string, workspace string) error {
	args := []string{"clone", "--depth", "1"}
	if gitRef != "" {
		args = append(args, "--branch", gitRef)
	}
	args = append(args, gitURL, workspace)
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone git workspace: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func buildSandboxEnv(env map[string]string) []string {
	sandboxEnv := make([]string, 0, len(env)+8)
	for _, key := range []string{"CURSOR_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY", "FRAMEVAL_CURSOR_COMMAND", "FRAMEVAL_GEMINI_COMMAND"} {
		if value := os.Getenv(key); value != "" {
			sandboxEnv = append(sandboxEnv, key+"="+value)
		}
	}
	for key, value := range env {
		sandboxEnv = append(sandboxEnv, key+"="+value)
	}
	return sandboxEnv
}

func (m *Manager) copyWorkspaceToContainer(ctx context.Context, containerID string, workspace string) error {
	archive, err := tarDirectory(workspace)
	if err != nil {
		return fmt.Errorf("archive workspace for sandbox: %w", err)
	}
	if err := m.dockerClient.CopyToContainer(ctx, containerID, "/workspace", bytes.NewReader(archive), container.CopyToContainerOptions{AllowOverwriteDirWithFile: true}); err != nil {
		return fmt.Errorf("copy workspace to sandbox container: %w", err)
	}
	return nil
}

func (m *Manager) copyWorkspaceFromContainer(ctx context.Context, containerID string, workspace string) error {
	reader, _, err := m.dockerClient.CopyFromContainer(ctx, containerID, "/workspace")
	if err != nil {
		return fmt.Errorf("copy workspace from sandbox container: %w", err)
	}
	defer reader.Close()
	if err := clearDirectory(workspace); err != nil {
		return fmt.Errorf("clear local workspace before restore: %w", err)
	}
	if err := extractTarToDir(reader, workspace, "workspace"); err != nil {
		return fmt.Errorf("restore workspace from sandbox container: %w", err)
	}
	return nil
}

func (m *Manager) readContainerLogs(ctx context.Context, containerID string) (string, error) {
	reader, err := m.dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", fmt.Errorf("read sandbox logs: %w", err)
	}
	defer reader.Close()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", fmt.Errorf("decode sandbox logs: %w", err)
	}
	return stdout.String() + stderr.String(), nil
}

func tarDirectory(root string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	err := filepath.WalkDir(root, func(currentPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentPath == root {
			return nil
		}
		relPath, err := filepath.Rel(root, currentPath)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if d.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}
		if _, err := writer.Write(content); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func extractTarToDir(reader io.Reader, targetDir string, prefixToStrip string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		normalized := strings.TrimPrefix(path.Clean(header.Name), "./")
		if normalized == "." || normalized == "" {
			continue
		}
		if prefixToStrip != "" && (normalized == prefixToStrip || strings.HasPrefix(normalized, prefixToStrip+"/")) {
			normalized = strings.TrimPrefix(strings.TrimPrefix(normalized, prefixToStrip), "/")
		}
		if normalized == "" {
			continue
		}
		destination := filepath.Join(targetDir, filepath.FromSlash(normalized))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			output, err := os.Create(destination)
			if err != nil {
				return err
			}
			if _, err := io.Copy(output, tarReader); err != nil {
				_ = output.Close()
				return err
			}
			if err := output.Close(); err != nil {
				return err
			}
		}
	}
}

func clearDirectory(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
