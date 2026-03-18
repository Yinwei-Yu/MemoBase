import { NavLink, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/auth';

const links = [
  { to: '/kbs', label: '知识库' },
  { to: '/sessions', label: '会话' },
  { to: '/ops', label: '运维状态' },
];

export default function TopNav() {
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();

  return (
    <header className="top-nav">
      <div className="brand">MemoBase MVP</div>
      <nav>
        {links.map((link) => (
          <NavLink key={link.to} to={link.to} className={({ isActive }) => (isActive ? 'active' : '')}>
            {link.label}
          </NavLink>
        ))}
      </nav>
      <div className="actions">
        <span>{user?.display_name ?? user?.username}</span>
        <button
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
