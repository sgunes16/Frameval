package models

type Task struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	Category         string     `json:"category"`
	TemplateKind     string     `json:"template_kind,omitempty"`
	WorkspaceMode    string     `json:"workspace_mode,omitempty"`
	WorkspaceGitURL  string     `json:"workspace_git_url,omitempty"`
	WorkspaceGitRef  string     `json:"workspace_git_ref,omitempty"`
	ComplexityScore  float64    `json:"complexity_score"`
	CodebaseType     string     `json:"codebase_type"`
	TaskPrompt       string     `json:"task_prompt"`
	TechnicalDetail  string     `json:"technical_details,omitempty"`
	SetupScript      string     `json:"setup_script,omitempty"`
	CodebasePath     string     `json:"codebase_path,omitempty"`
	TaskRootPath     string     `json:"task_root_path,omitempty"`
	IsBuiltin        bool       `json:"is_builtin"`
	CreatedAt        string     `json:"created_at,omitempty"`
	TestCases        []TestCase `json:"test_cases,omitempty"`
}

type TestCase struct {
	ID             string `json:"id"`
	TaskID         string `json:"task_id,omitempty"`
	Name           string `json:"name"`
	TestCommand    string `json:"test_command"`
	ExpectedResult string `json:"expected_result,omitempty"`
	Ordering       int    `json:"ordering"`
}
