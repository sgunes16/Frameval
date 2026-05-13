// AgentDx diagnostic profile types — mirror engine/pkg/diagnostic.
// Field names in snake_case to match the gRPC + JSON wire format.

export type Fingerprint = {
  planning_depth: number;
  tool_call_diversity: number;
  self_validation_rate: number;
  backtrack_rate: number;
  file_focus: number;
  recovery_latency: number;
  premature_completion: number;
  turn_efficiency: number;
  context_reference_rate: number;
  idle_thinking_ratio: number;
};

export type ToolFailure = {
  turn_index: number;
  tool_name: string;
  message: string;
};

export type Symptoms = {
  tests_passed: number;
  tests_failed: number;
  tests_total: number;
  compile_failed: boolean;
  lint_errors?: string[];
  tool_failures?: ToolFailure[];
  last_error_message?: string;
  files_touched?: string[];
  files_created?: string[];
  files_deleted?: string[];
  last_assistant_claim?: string;
  declared_completion: boolean;
  acknowledged_failure: boolean;
  time_to_first_error: number;
  timeout_hit: boolean;
  wall_clock_seconds: number;
  unexpected_files_modified?: string[];
};

export type ErrorEvent = {
  turn_index: number;
  type: 'tool_failure' | 'test_failure' | 'stderr' | 'compile_error';
  tool_name?: string;
  message: string;
};

export type RecoveryProfile = {
  error_events?: ErrorEvent[];
  error_acknowledgment_rate: number;
  correction_latency_mean: number;
  correction_success_rate: number;
  silent_skip_count: number;
};

export type FailureCode =
  | 'NONE' | 'HAL_API' | 'HAL_FILE' | 'DEP_MISS'
  | 'STOP_EARLY' | 'STOP_GIVEUP' | 'LOOP_INF' | 'WRONG_ABS'
  | 'MISREAD' | 'ENV_ERR' | 'SCOPE_DRIFT' | 'TIMEOUT' | 'SILENT_SKIP';

export type EvidenceSpan = {
  code: FailureCode;
  quote: string;
  turn_index: number;
};

export type FailureClassification = {
  primary: FailureCode;
  secondary?: FailureCode[];
  evidence?: EvidenceSpan[];
  confidence: number;
  rationale?: string;
};

export type Diagnostic = {
  id: string;
  run_id: string;
  fingerprint: Fingerprint;
  symptoms: Symptoms;
  recovery: RecoveryProfile;
  failure_label?: FailureClassification | null;
  classifier_model?: string;
  classifier_latency_ms?: number;
  created_at?: string;
};

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
  patch?: string;
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
  external_source?: string;
  external_id?: string;
  external_url?: string;
  metadata?: Record<string, unknown>;
  complexity_score: number;
  codebase_type: string;
  task_prompt: string;
  setup_script?: string;
  codebase_path?: string;
  task_root_path?: string;
  test_cases?: Array<{ id?: string; name: string; test_command: string; expected_result?: string; visibility?: string; timeout_seconds?: number; setup_script?: string; ordering: number }>;
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

export type HarnessInfo = {
  id: string;
  name: string;
  description: string;
};

export type ExecutorInfo = {
  id: string;
  modes: string[];
};

export type LaunchDiagnosticRequest = {
  task_id: string;
  executor_id: string;
  harness_ids: string[];
  model?: string;
  runs_per_variant?: number;
  timeout_seconds?: number;
  name?: string;
};

export type LaunchDiagnosticResponse = {
  experiment_id: string;
};

export type APIKey = {
  id: string;
  provider: string;
  redacted_key: string;
};

export type DockerStatus = {
  healthy: boolean;
  mode: string;
  sandbox_image: string;
  message?: string;
  api_version?: string;
};

export type QueueStatus = {
  depth: number;
  active_workers: number;
  max_workers: number;
};
