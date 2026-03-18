import { Outlet } from 'react-router-dom';
import TopNav from './TopNav';

export default function AppLayout() {
  return (
    <div className="app-shell">
      <TopNav />
      <main className="page-shell">
        <Outlet />
      </main>
    </div>
  );
}
