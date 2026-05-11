import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import ChatPage from './ChatPage';
import * as apiClient from '../lib/api/client';

vi.mock('../lib/api/client', () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiUpload: vi.fn(),
  apiDelete: vi.fn(),
  apiPatch: vi.fn(),
  client: { get: vi.fn() },
}));

function renderChatPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/chat/kb_001']}>
        <Routes>
          <Route path="/chat/:kbId" element={<ChatPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ChatPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.apiGet).mockResolvedValue({ items: [], pagination: { page: 1, page_size: 200, total: 0 } });
  });

  it('renders heading and empty state', async () => {
    renderChatPage();
    expect(screen.getByText('智能问答')).toBeInTheDocument();
    expect(screen.getByText('输入问题后，这里会按一问一答展示聊天记录。')).toBeInTheDocument();
  });

  it('renders textarea and send button', () => {
    renderChatPage();
    expect(screen.getByPlaceholderText('输入你的问题...')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '发送' })).toBeInTheDocument();
  });

  it('disables send button when question is empty', () => {
    renderChatPage();
    expect(screen.getByRole('button', { name: '发送' })).toBeDisabled();
  });

  it('enables send button when question is typed', async () => {
    const user = userEvent.setup();
    renderChatPage();
    const textarea = screen.getByPlaceholderText('输入你的问题...');
    await user.type(textarea, '什么是死锁？');
    expect(screen.getByRole('button', { name: '发送' })).not.toBeDisabled();
  });

  it('shows document link', () => {
    renderChatPage();
    expect(screen.getByRole('link', { name: '文档' })).toHaveAttribute('href', '/kbs/kb_001/documents');
  });

  it('renders messages when loaded', async () => {
    vi.mocked(apiClient.apiGet).mockImplementation(async (url: string) => {
      if (url.includes('/messages')) {
        return {
          items: [
            { message_id: 'msg_1', session_id: 'sess_1', role: 'user', content: '什么是死锁？', created_at: '' },
            { message_id: 'msg_2', session_id: 'sess_1', role: 'assistant', content: '死锁是...', created_at: '' },
          ],
          pagination: { page: 1, page_size: 200, total: 2 },
        };
      }
      return { items: [], pagination: { page: 1, page_size: 200, total: 0 } };
    });
    renderChatPage();
    await waitFor(() => {
      expect(screen.getByText('什么是死锁？')).toBeInTheDocument();
    });
  });

  it('shows error box on mutation error', async () => {
    vi.mocked(apiClient.apiPost).mockRejectedValue(new Error('MODEL_UNAVAILABLE: ollama chat failed'));
    const user = userEvent.setup();
    renderChatPage();
    const textarea = screen.getByPlaceholderText('输入你的问题...');
    await user.type(textarea, 'test question');
    await user.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() => {
      expect(screen.getByText(/MODEL_UNAVAILABLE/)).toBeInTheDocument();
    });
  });

  it('shows loading state during submission', async () => {
    vi.mocked(apiClient.apiPost).mockImplementation(() => new Promise(() => {}));
    const user = userEvent.setup();
    renderChatPage();
    const textarea = screen.getByPlaceholderText('输入你的问题...');
    await user.type(textarea, 'test');
    await user.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() => {
      expect(screen.getByText('生成中...')).toBeInTheDocument();
    });
  });
});
