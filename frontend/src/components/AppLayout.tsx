import { useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import TopNav from './TopNav';

const links = [
  { to: '/kbs', label: 'Knowledge Bases', note: '管理知识库资产', short: 'KB' },
  { to: '/sessions', label: 'Sessions', note: '查看会话与消息', short: 'SE' },
  { to: '/ops', label: 'System Health', note: '实时状态监控', short: 'OP' },
];

export default function AppLayout() {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div className="app-shell">
      <TopNav />
      <div className={`workspace-shell ${collapsed ? 'is-collapsed' : ''}`}>
        <aside className="side-nav">
          <div className="side-nav-head">
            <div className="side-nav-heading">
              <p className="side-nav-title">Workspace</p>
              <p className="side-nav-note">KnowledgeAI Console</p>
            </div>
            <button
              type="button"
              className="side-toggle"
              aria-label={collapsed ? '展开侧边栏' : '收起侧边栏'}
              aria-pressed={collapsed}
              onClick={() => setCollapsed((prev) => !prev)}
            >
              {collapsed ? '>' : '<'}
            </button>
          </div>
          <nav className="side-nav-links">
            {links.map((link) => (
              <NavLink
                key={link.to}
                to={link.to}
                className={({ isActive }) => `side-link ${isActive ? 'active' : ''}`}
                title={link.label}
              >
                <span className="side-link-mark">{link.short}</span>
                <span className="side-link-copy">
                  <span>{link.label}</span>
                  <small>{link.note}</small>
                </span>
              </NavLink>
            ))}
          </nav>
        </aside>
        <main className="page-shell">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
