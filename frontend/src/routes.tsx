import { Navigate, Routes, Route } from 'react-router-dom';
import { TasksPage } from './pages/tasks';
import { TaskDetailPage } from './pages/tasks/detail';
import { NewTaskPage } from './pages/tasks/new';
import { ExperimentsPage } from './pages/experiments';
import { ExperimentMonitorPage } from './pages/experiments/monitor';
import { RunInspectPage } from './pages/runs/inspect';
import { RunGradingPage } from './pages/runs/grading';
import { DiagnosticComparePage } from './pages/diagnostic/compare';
import { DiagnosticLaunchPage } from './pages/diagnostic/launch';
import { SettingsPage } from './pages/settings';
import { RubricsPage } from './pages/rubrics';

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/experiments" replace />} />
      <Route path="/tasks" element={<TasksPage />} />
      <Route path="/tasks/new" element={<NewTaskPage />} />
      <Route path="/tasks/:id" element={<TaskDetailPage />} />
      <Route path="/experiments" element={<ExperimentsPage />} />
      <Route path="/experiments/:id/monitor" element={<ExperimentMonitorPage />} />
      <Route path="/runs/:id/inspect" element={<RunInspectPage />} />
      <Route path="/runs/:id/grading" element={<RunGradingPage />} />
      <Route path="/diagnostic/launch" element={<DiagnosticLaunchPage />} />
      <Route path="/diagnostic/compare" element={<DiagnosticComparePage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/rubrics" element={<RubricsPage />} />
      <Route path="*" element={<Navigate to="/experiments" replace />} />
    </Routes>
  );
}
