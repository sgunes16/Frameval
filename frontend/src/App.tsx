import { Sidebar } from './components/layout/sidebar';
import { Header } from './components/layout/header';
import { AppRoutes } from './routes';

export default function App() {
  return (
    <div className="flex min-h-screen bg-bg text-fg">
      <Sidebar />
      <div className="flex min-h-screen flex-1 flex-col">
        <Header />
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-[1180px]">
            <AppRoutes />
          </div>
        </main>
      </div>
    </div>
  );
}
