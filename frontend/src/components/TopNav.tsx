import { useEffect, useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/auth';

const quickLinks = [
  { to: '/kbs', label: '知识库' },
  { to: '/sessions', label: '会话' },
  { to: '/ops', label: '运维状态' },
];

function getInitialTheme(): 'light' | 'dark' {
  if (typeof window === 'undefined') return 'light';
  const stored = localStorage.getItem('theme');
  if (stored === 'dark' || stored === 'light') return stored;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

export default function TopNav() {
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();
  const [theme, setTheme] = useState<'light' | 'dark'>(getInitialTheme);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
  }, [theme]);

  function toggleTheme() {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'));
  }

  return (
    <header className="top-nav">
      <div className="brand">
        <span className="brand-mark" aria-hidden="true">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M3 2h4v5.5L11 2h2l-4.5 5.5L13 14h-2l-4-5.5V14H3V2z" fill="currentColor"/>
            <circle cx="12.5" cy="3.5" r="1.5" fill="currentColor" opacity="0.6"/>
          </svg>
        </span>
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
          type="button"
          className="theme-toggle"
          onClick={toggleTheme}
          aria-label={theme === 'dark' ? '切换为浅色模式' : '切换为深色模式'}
        >
          {theme === 'dark' ? '\u2600' : '\u263E'}
        </button>
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
