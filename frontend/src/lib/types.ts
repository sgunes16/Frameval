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
  harness_id?: string;
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

/**
 * BlockKind classifies what a ParsedTurn's payload represents. Used by
 * Inspector V2 to colour the turn card's left bar and by Compare V2 when
 * computing anchors.
 *
 * The empty string ("") is reserved for legacy data — transcripts written
 * before the schema extension don't have this stamped and UIs must treat
 * the absence as "we don't know" rather than "definitely text".
 */
export type BlockKind = 'thinking' | 'text' | 'tool_use' | 'tool_result' | 'system' | '';

export type ParsedTurn = {
  role: string;
  content: string;
  timestamp?: string;
  stage?: string;
  // Inspector V2 fields — zero / undefined on legacy transcripts.
  turn_index?: number;
  block_kind?: BlockKind;
  tool_use_id?: string;
  parent_turn_index?: number;
  tool_name?: string;
  files_touched?: string[];
  tool_output?: string;
  duration_ms?: number;
  tokens_in?: number;
  tokens_out?: number;
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
  parsed_turns?: ParsedTurn[];
};

export type Grade = {
  // composite — weighted blend driven by `composite_weights` in the
  // experiment config (defaults: code 0.3 / judge 0.3 / process 0.2 /
  // spec 0.2). Range 0..10 since each child axis tops at 10.
  composite_score: number;

  // --- Code grading (deterministic) ---
  test_pass_rate: number;
  test_pass_count?: number;
  test_fail_count?: number;
  lint_score: number;
  type_check_pass?: boolean;
  file_state_valid?: boolean;

  // --- Process metrics (transcript-derived) ---
  turn_count?: number;
  total_tokens?: number;
  cost_usd?: number;
  token_efficiency: number;
  backtrack_count?: number;
  self_validation_rate?: number;
  premature_completion?: boolean;
  idle_turns?: number;
  error_recovery_count?: number;
  tool_call_accuracy?: number;
  context_utilization: number;

  // --- LLM-as-Judge rubric (cross-model) ---
  judge_correctness: number;
  judge_maintainability?: number;
  judge_completeness?: number;
  judge_best_practices?: number;
  judge_error_handling?: number;
  judge_irr_alpha?: number;

  // --- Spec / instruction adherence ---
  spec_instruction_compliance: number;
  spec_constraint_violations?: number;
  spec_convention_adherence?: number;

  graded_at?: string;
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

export type LLMProvider = 'openrouter' | 'zai' | 'ollama' | 'openai' | 'anthropic';

export type LLMSettings = {
  provider: LLMProvider;
  model: string;
  enabled: boolean;
  api_key_present: boolean;
};
