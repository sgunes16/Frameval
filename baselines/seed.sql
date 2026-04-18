INSERT INTO baselines (id, name, description, source, artifact_type, artifact_content, task_id, model, agent_cli, total_runs, evaluated_at)
VALUES
  ('baseline-control-jwt', 'JWT Control', 'Starter control baseline for JWT task', 'control', 'claude_md', '', 'jwt-auth-express', 'gpt-5.4', 'cursor', 5, '2026-04-08T00:00:00Z'),
  ('baseline-control-pagination', 'Pagination Control', 'Starter control baseline for pagination task', 'control', 'cursorrules', '', 'api-pagination', 'gpt-5.4', 'cursor', 5, '2026-04-08T00:00:00Z');
