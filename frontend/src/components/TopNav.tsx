import { NavLink, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/auth';

const quickLinks = [
  { to: '/kbs', label: '知识库' },
  { to: '/sessions', label: '会话' },
  { to: '/ops', label: '运维状态' },
];

export default function TopNav() {
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();

  return (
    <header className="top-nav">
      <div className="brand">
        <span className="brand-mark">◆</span>
        <span>KnowledgeAI</span>
      </div>
      <nav className="top-links">
        {quickLinks.map((link) => (
          <NavLink key={link.to} to={link.to} className={({ isActive }) => (isActive ? 'active' : '')}>
            {link.label}
          </NavLink>
        ))}
      </nav>
      <div className="actions">
        <div className="user-chip">{user?.display_name ?? user?.username}</div>
        <button
          className="ghost-btn"
          onClick={() => {
            logout();
            navigate('/login');
          }}
        >
          退出
        </button>
      </div>
    </header>
  );
}
