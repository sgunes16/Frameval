import { Routes, Route } from 'react-router-dom';
import { DashboardPage } from './pages/dashboard';
import { ArtifactsPage } from './pages/artifacts';
import { ArtifactDetailPage } from './pages/artifacts/detail';
import { TasksPage } from './pages/tasks';
import { TaskDetailPage } from './pages/tasks/detail';
import { NewTaskPage } from './pages/tasks/new';
import { BaselinesPage } from './pages/baselines';
import { BaselineDetailPage } from './pages/baselines/detail';
import { ExperimentsPage } from './pages/experiments';
import { NewExperimentPage } from './pages/experiments/new';
import { ExperimentMonitorPage } from './pages/experiments/monitor';
import { ExperimentResultsPage } from './pages/experiments/results';
import { SettingsPage } from './pages/settings';

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<DashboardPage />} />
      <Route path="/artifacts" element={<ArtifactsPage />} />
      <Route path="/artifacts/:id" element={<ArtifactDetailPage />} />
      <Route path="/tasks" element={<TasksPage />} />
      <Route path="/tasks/new" element={<NewTaskPage />} />
      <Route path="/tasks/:id" element={<TaskDetailPage />} />
      <Route path="/baselines" element={<BaselinesPage />} />
      <Route path="/baselines/:id" element={<BaselineDetailPage />} />
      <Route path="/experiments" element={<ExperimentsPage />} />
      <Route path="/experiments/new" element={<NewExperimentPage />} />
      <Route path="/experiments/:id/monitor" element={<ExperimentMonitorPage />} />
      <Route path="/experiments/:id/results" element={<ExperimentResultsPage />} />
      <Route path="/settings" element={<SettingsPage />} />
    </Routes>
  );
}
