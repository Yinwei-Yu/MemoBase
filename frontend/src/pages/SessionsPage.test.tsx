import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import SessionsPage from './SessionsPage';
import * as apiClient from '../lib/api/client';

vi.mock('../lib/api/client', () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiUpload: vi.fn(),
  apiDelete: vi.fn(),
  apiPatch: vi.fn(),
  client: { get: vi.fn() },
}));

function renderSessionsPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/sessions']}>
        <SessionsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const mockSessions = {
  items: [
    { session_id: 'sess_1', kb_id: 'kb_001', title: '操作系统复习', created_at: '', updated_at: '' },
    { session_id: 'sess_2', kb_id: 'kb_002', title: '数据库问答', created_at: '', updated_at: '' },
  ],
  pagination: { page: 1, page_size: 100, total: 2 },
};

const mockMessages = {
  items: [
    { message_id: 'msg_1', session_id: 'sess_1', role: 'user', content: '什么是死锁？', created_at: '' },
    { message_id: 'msg_2', session_id: 'sess_1', role: 'assistant', content: '死锁是多个进程互相等待...', created_at: '' },
  ],
  pagination: { page: 1, page_size: 200, total: 2 },
};

describe('SessionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.apiGet).mockResolvedValue(mockSessions);
  });

  it('renders heading', () => {
    renderSessionsPage();
    expect(screen.getByText('会话管理')).toBeInTheDocument();
  });

  it('renders session list', async () => {
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getByText('操作系统复习')).toBeInTheDocument();
      expect(screen.getByText('数据库问答')).toBeInTheDocument();
    });
  });

  it('shows kb_id for each session', async () => {
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getByText('kb: kb_001')).toBeInTheDocument();
      expect(screen.getByText('kb: kb_002')).toBeInTheDocument();
    });
  });

  it('shows empty state when no sessions', async () => {
    vi.mocked(apiClient.apiGet).mockResolvedValue({ items: [], pagination: { page: 1, page_size: 100, total: 0 } });
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getByText('暂无会话')).toBeInTheDocument();
    });
  });

  it('shows loading state', () => {
    vi.mocked(apiClient.apiGet).mockImplementation(() => new Promise(() => {}));
    renderSessionsPage();
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('shows error on load failure', async () => {
    vi.mocked(apiClient.apiGet).mockRejectedValue(new Error('INTERNAL: failed'));
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getByText(/INTERNAL/)).toBeInTheDocument();
    });
  });

  it('shows action buttons for each session', async () => {
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getAllByText('查看消息')).toHaveLength(2);
      expect(screen.getAllByText('删除')).toHaveLength(2);
    });
  });

  it('shows placeholder when no session selected', () => {
    renderSessionsPage();
    expect(screen.getByText('选择一个会话查看消息')).toBeInTheDocument();
  });

  it('loads messages when session is selected', async () => {
    vi.mocked(apiClient.apiGet).mockImplementation(async (url: string) => {
      if (url.includes('/messages')) {
        return mockMessages;
      }
      return mockSessions;
    });
    const user = userEvent.setup();
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getByText('操作系统复习')).toBeInTheDocument();
    });
    const viewButtons = screen.getAllByText('查看消息');
    await user.click(viewButtons[0]);
    await waitFor(() => {
      expect(screen.getByText('什么是死锁？')).toBeInTheDocument();
      expect(screen.getByText('死锁是多个进程互相等待...')).toBeInTheDocument();
    });
  });

  it('shows role labels in messages', async () => {
    vi.mocked(apiClient.apiGet).mockImplementation(async (url: string) => {
      if (url.includes('/messages')) {
        return mockMessages;
      }
      return mockSessions;
    });
    const user = userEvent.setup();
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getByText('操作系统复习')).toBeInTheDocument();
    });
    await user.click(screen.getAllByText('查看消息')[0]);
    await waitFor(() => {
      expect(screen.getByText('你：')).toBeInTheDocument();
      expect(screen.getByText('助手：')).toBeInTheDocument();
    });
  });

  it('delete button triggers mutation', async () => {
    vi.mocked(apiClient.apiDelete).mockResolvedValue({ deleted: true });
    const user = userEvent.setup();
    renderSessionsPage();
    await waitFor(() => {
      expect(screen.getByText('操作系统复习')).toBeInTheDocument();
    });
    const deleteButtons = screen.getAllByText('删除');
    await user.click(deleteButtons[0]);
    expect(apiClient.apiDelete).toHaveBeenCalledWith('/sessions/sess_1');
  });
});
