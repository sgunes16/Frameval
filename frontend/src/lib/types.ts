export type Variant = {
  id: string;
  experiment_id: string;
  name: string;
  description?: string;
  is_control: boolean;
  ordering: number;
  artifact_versions?: ArtifactVersion[];
};

export type ArtifactVersion = {
  id: string;
  variant_id: string;
  artifact_type: string;
  source_kind?: string;
  display_name?: string;
  source_ref?: string;
  file_path: string;
  content: string;
  content_hash: string;
  dimensions?: Record<string, unknown>;
  created_at: string;
};

export type Experiment = {
  id: string;
  name: string;
  description?: string;
  status: string;
  task_id: string;
  workspace_source_type?: string;
  local_path?: string;
  git_url?: string;
  git_ref?: string;
  model: string;
  agent_cli: string;
  execution_mode: string;
  runs_per_variant: number;
  temperature: number;
  timeout_seconds: number;
  max_concurrent: number;
  estimated_cost_usd?: number;
  created_at: string;
  variants?: Variant[];
  stats?: ExperimentStat[];
};

export type Run = {
  id: string;
  experiment_id: string;
  variant_id: string;
  run_number: number;
  status: string;
  started_at?: string;
  completed_at?: string;
  duration_seconds?: number;
  error_message?: string;
  transcript?: Transcript;
  grade?: Grade;
};

export type Transcript = {
  id: string;
  run_id: string;
  raw_output: string;
  filesystem_diff?: string;
  total_turns: number;
  total_tokens: number;
  output_files?: Array<{ path: string; content: string }>;
};

export type Grade = {
  composite_score: number;
  test_pass_rate: number;
  lint_score: number;
  token_efficiency: number;
  context_utilization: number;
  turn_count?: number;
  total_tokens?: number;
  judge_correctness: number;
  spec_instruction_compliance: number;
  test_results?: Array<{ name: string; passed: boolean; output: string }>;
};

export type Task = {
  id: string;
  name: string;
  description: string;
  category: string;
  template_kind?: string;
  workspace_mode?: string;
  workspace_git_url?: string;
  workspace_git_ref?: string;
  complexity_score: number;
  codebase_type: string;
  task_prompt: string;
  setup_script?: string;
  codebase_path?: string;
  task_root_path?: string;
  test_cases?: Array<{ id?: string; name: string; test_command: string; expected_result?: string; ordering: number }>;
};

export type Baseline = {
  id: string;
  name: string;
  description?: string;
  source: string;
  task_id: string;
  model: string;
  agent_cli: string;
  total_runs: number;
  evaluated_at: string;
};

export type ModelConfig = {
  id: string;
  provider: string;
  model_id: string;
  display_name: string;
  input_price_per_1k: number;
  output_price_per_1k: number;
};

export type AgentInfo = {
  name: string;
  modes: string[];
  available: boolean;
};

export type APIKey = {
  id: string;
  provider: string;
  redacted_key: string;
};

export type ExperimentStat = {
  id?: string;
  metric_name: string;
  variant_a_id: string;
  variant_b_id: string;
  mean_a: number;
  mean_b: number;
  p_value: number;
  is_significant: boolean;
};

export type CatalogExtension = {
  id: string;
  name: string;
  description: string;
  author?: string;
  version?: string;
  download_url: string;
  repository?: string;
  homepage?: string;
  documentation?: string;
  license?: string;
  tags?: string[];
  verified?: boolean;
};

export type CatalogResponse = {
  schema_version: string;
  updated_at: string;
  extensions: CatalogExtension[];
};
