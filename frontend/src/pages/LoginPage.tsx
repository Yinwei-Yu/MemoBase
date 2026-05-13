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
      <form className="card login-card stack" onSubmit={onSubmit}>
        <p className="eyebrow">Welcome to KnowledgeAI</p>
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
  );
}
