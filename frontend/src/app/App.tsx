import { Navigate, Route, Routes } from 'react-router-dom';
import AppLayout from '../components/AppLayout';
import AppErrorBoundary from '../components/AppErrorBoundary';
import LoginPage from '../pages/LoginPage';
import KnowledgeBasePage from '../pages/KnowledgeBasePage';
import DocumentsPage from '../pages/DocumentsPage';
import ChatPage from '../pages/ChatPage';
import SessionsPage from '../pages/SessionsPage';
import OpsPage from '../pages/OpsPage';
import { useAuthStore } from '../stores/auth';

function ProtectedRoute({ children }: { children: JSX.Element }) {
  const token = useAuthStore((s) => s.token);
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  return (
    <AppErrorBoundary>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          element={
            <ProtectedRoute>
              <AppLayout />
            </ProtectedRoute>
          }
        >
          <Route path="/" element={<Navigate to="/kbs" replace />} />
          <Route path="/kbs" element={<KnowledgeBasePage />} />
          <Route path="/kbs/:kbId/documents" element={<DocumentsPage />} />
          <Route path="/chat/:kbId" element={<ChatPage />} />
          <Route path="/sessions" element={<SessionsPage />} />
          <Route path="/ops" element={<OpsPage />} />
        </Route>
      </Routes>
    </AppErrorBoundary>
  );
}
