package models

type Task struct {
	ID              string     `json:"id" yaml:"id"`
	Name            string     `json:"name" yaml:"name"`
	Description     string     `json:"description" yaml:"description"`
	Category        string     `json:"category" yaml:"category"`
	TemplateKind    string     `json:"template_kind,omitempty" yaml:"template_kind,omitempty"`
	WorkspaceMode   string     `json:"workspace_mode,omitempty" yaml:"workspace_mode,omitempty"`
	WorkspaceGitURL string     `json:"workspace_git_url,omitempty" yaml:"workspace_git_url,omitempty"`
	WorkspaceGitRef string     `json:"workspace_git_ref,omitempty" yaml:"workspace_git_ref,omitempty"`
	ComplexityScore float64    `json:"complexity_score" yaml:"complexity_score"`
	CodebaseType    string     `json:"codebase_type" yaml:"codebase_type"`
	TaskPrompt      string     `json:"task_prompt" yaml:"task_prompt"`
	TechnicalDetail string     `json:"technical_details,omitempty" yaml:"technical_details,omitempty"`
	SetupScript     string     `json:"setup_script,omitempty" yaml:"setup_script,omitempty"`
	CodebasePath    string     `json:"codebase_path,omitempty" yaml:"codebase_path,omitempty"`
	TaskRootPath    string     `json:"task_root_path,omitempty" yaml:"task_root_path,omitempty"`
	IsBuiltin       bool       `json:"is_builtin" yaml:"is_builtin"`
	CreatedAt       string     `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	TestCases       []TestCase `json:"test_cases,omitempty" yaml:"test_cases,omitempty"`
}

type TestCase struct {
	ID             string `json:"id" yaml:"id,omitempty"`
	TaskID         string `json:"task_id,omitempty" yaml:"task_id,omitempty"`
	Name           string `json:"name" yaml:"name"`
	TestCommand    string `json:"test_command" yaml:"test_command"`
	ExpectedResult string `json:"expected_result,omitempty" yaml:"expected_result,omitempty"`
	Ordering       int    `json:"ordering" yaml:"ordering"`
}
