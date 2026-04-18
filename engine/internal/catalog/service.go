package catalog

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

const defaultCatalogURL = "https://raw.githubusercontent.com/github/spec-kit/main/extensions/catalog.community.json"

type document struct {
	SchemaVersion string                             `json:"schema_version"`
	UpdatedAt     string                             `json:"updated_at"`
	Extensions    map[string]models.CatalogExtension `json:"extensions"`
}

func LoadSpecKitCatalog(ctx context.Context) (*models.CatalogListResponse, error) {
	raw, err := loadCatalogBytes(ctx)
	if err != nil {
		return nil, err
	}
	var parsed document
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	extensions := make([]models.CatalogExtension, 0, len(parsed.Extensions))
	for id, extension := range parsed.Extensions {
		if extension.ID == "" {
			extension.ID = id
		}
		extensions = append(extensions, extension)
	}
	sort.Slice(extensions, func(i, j int) bool { return extensions[i].Name < extensions[j].Name })
	return &models.CatalogListResponse{
		SchemaVersion: parsed.SchemaVersion,
		UpdatedAt:     parsed.UpdatedAt,
		Extensions:    extensions,
	}, nil
}

func SelectExtensions(ctx context.Context, ids []string) ([]models.CatalogExtension, error) {
	catalog, err := LoadSpecKitCatalog(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[string]models.CatalogExtension{}
	for _, extension := range catalog.Extensions {
		byID[extension.ID] = extension
	}
	selected := make([]models.CatalogExtension, 0, len(ids))
	for _, id := range ids {
		extension, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("catalog extension %q not found", id)
		}
		selected = append(selected, extension)
	}
	return selected, nil
}

func DownloadExtensionFiles(ctx context.Context, extension models.CatalogExtension) ([]models.ArtifactRequest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, extension.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build extension download request: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download extension archive: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("download extension archive: status %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read extension archive: %w", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open extension archive: %w", err)
	}
	files := make([]models.ArtifactRequest, 0)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		normalizedPath := normalizeArchivePath(file.Name)
		if normalizedPath == "" || !isImportablePath(normalizedPath) {
			continue
		}
		if file.FileInfo().Size() > 512*1024 {
			continue
		}
		content, err := readArchiveFile(file)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(content) {
			continue
		}
		files = append(files, models.ArtifactRequest{
			ArtifactType: inferArtifactType(normalizedPath),
			SourceKind:   "catalog_extension",
			DisplayName:  extension.Name,
			SourceRef:    extension.ID,
			FilePath:     normalizedPath,
			Content:      string(content),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].FilePath < files[j].FilePath })
	return files, nil
}

func loadCatalogBytes(ctx context.Context) ([]byte, error) {
	if filePath := os.Getenv("FRAMEVAL_SPEC_KIT_CATALOG_PATH"); filePath != "" {
		raw, err := os.ReadFile(filePath)
		if err == nil {
			return raw, nil
		}
	}
	url := os.Getenv("FRAMEVAL_SPEC_KIT_CATALOG_URL")
	if url == "" {
		url = defaultCatalogURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build catalog request: %w", err)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch catalog: status %d", response.StatusCode)
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read catalog response: %w", err)
	}
	return raw, nil
}

func normalizeArchivePath(name string) string {
	parts := strings.Split(strings.TrimLeft(path.Clean(strings.ReplaceAll(name, "\\", "/")), "./"), "/")
	if len(parts) <= 1 {
		if len(parts) == 1 {
			return parts[0]
		}
		return ""
	}
	return strings.Join(parts[1:], "/")
}

func isImportablePath(filePath string) bool {
	lower := strings.ToLower(filePath)
	disallowedSuffixes := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".mp4", ".mov", ".pdf", ".zip", ".gz", ".tar", ".tgz", ".so", ".node", ".dll", ".exe"}
	for _, suffix := range disallowedSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}
	if strings.HasSuffix(lower, ".ds_store") {
		return false
	}
	return true
}

func inferArtifactType(filePath string) string {
	base := path.Base(filePath)
	switch base {
	case "CLAUDE.md":
		return "claude_md"
	case "AGENTS.md":
		return "agents_md"
	case ".cursorrules":
		return "cursorrules"
	default:
		return "supporting_file"
	}
}

func readArchiveFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open archive file %s: %w", file.Name, err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read archive file %s: %w", file.Name, err)
	}
	return content, nil
}
