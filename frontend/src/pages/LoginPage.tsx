import { FormEvent, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiPost } from '../lib/api/client';
import { useAuthStore } from '../stores/auth';

type LoginResponse = {
  access_token: string;
  user: {
    user_id: string;
    username: string;
    display_name: string;
  };
};

export default function LoginPage() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);
  const [username, setUsername] = useState('demo');
  const [password, setPassword] = useState('demo123');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError('');
    setLoading(true);
    try {
      const data = await apiPost<LoginResponse, { username: string; password: string }>('/auth/login', {
        username,
        password,
      });
      setAuth(data.access_token, data.user);
      navigate('/kbs');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      {/* Left: Brand / Illustration Panel */}
      <div className="login-brand-panel">
        <div className="login-float-shape login-float-shape--1" />
        <div className="login-float-shape login-float-shape--2" />
        <div className="login-float-shape login-float-shape--3" />
        <div className="login-brand-content">
          <div className="login-brand-mark">
            <svg width="32" height="32" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M3 2h4v5.5L11 2h2l-4.5 5.5L13 14h-2l-4-5.5V14H3V2z" fill="currentColor"/>
              <circle cx="12.5" cy="3.5" r="1.5" fill="currentColor" opacity="0.6"/>
            </svg>
          </div>
          <h2>KnowledgeAI</h2>
          <p>智能知识管理与 RAG 问答平台。构建、管理并对话你的知识资产。</p>
        </div>
      </div>

      {/* Right: Login Form Panel */}
      <div className="login-form-panel">
        <form className="card login-card stack" onSubmit={onSubmit}>
          <p className="eyebrow">Welcome back</p>
          <h1>登录控制台</h1>
          <p className="muted system-tip">MVP 默认账户: demo / demo123</p>
          <label>
            用户名
            <input value={username} onChange={(e) => setUsername(e.target.value)} required />
          </label>
          <label>
            密码
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </label>
          {error && <div className="error-box">{error}</div>}
          <button disabled={loading} type="submit">
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
      </div>
    </div>
  );
}
