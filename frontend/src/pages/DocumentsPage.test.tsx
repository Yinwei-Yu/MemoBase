import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import DocumentsPage from './DocumentsPage';
import * as apiClient from '../lib/api/client';

vi.mock('../lib/api/client', () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiUpload: vi.fn(),
  apiDelete: vi.fn(),
  apiPatch: vi.fn(),
  client: { get: vi.fn() },
}));

function renderDocsPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/kbs/kb_001/documents']}>
        <Routes>
          <Route path="/kbs/:kbId/documents" element={<DocumentsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const mockDocs = {
  items: [
    { doc_id: 'doc_1', kb_id: 'kb_001', title: 'OS笔记', file_name: 'os-notes.md', status: 'indexed', created_at: '', updated_at: '' },
    { doc_id: 'doc_2', kb_id: 'kb_001', title: '实验报告', file_name: 'lab.txt', status: 'pending', created_at: '', updated_at: '' },
  ],
  pagination: { page: 1, page_size: 100, total: 2 },
};

describe('DocumentsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.apiGet).mockResolvedValue(mockDocs);
  });

  it('renders heading and upload form', async () => {
    renderDocsPage();
    expect(screen.getByText('文档与索引')).toBeInTheDocument();
    expect(screen.getByText('文档上传与索引')).toBeInTheDocument();
    expect(screen.getByText('上传并建立索引')).toBeInTheDocument();
  });

  it('renders document list', async () => {
    renderDocsPage();
    await waitFor(() => {
      expect(screen.getByText('OS笔记')).toBeInTheDocument();
      expect(screen.getByText('os-notes.md')).toBeInTheDocument();
      expect(screen.getByText('实验报告')).toBeInTheDocument();
    });
  });

  it('shows empty state when no documents', async () => {
    vi.mocked(apiClient.apiGet).mockResolvedValue({ items: [], pagination: { page: 1, page_size: 100, total: 0 } });
    renderDocsPage();
    await waitFor(() => {
      expect(screen.getByText('暂无文档')).toBeInTheDocument();
    });
  });

  it('shows loading state', () => {
    vi.mocked(apiClient.apiGet).mockImplementation(() => new Promise(() => {}));
    renderDocsPage();
    expect(screen.getByText('加载中...')).toBeInTheDocument();
  });

  it('shows error on load failure', async () => {
    vi.mocked(apiClient.apiGet).mockRejectedValue(new Error('INTERNAL: failed'));
    renderDocsPage();
    await waitFor(() => {
      expect(screen.getByText(/INTERNAL/)).toBeInTheDocument();
    });
  });

  it('shows file type hint', () => {
    renderDocsPage();
    expect(screen.getByText('当前仅支持 .txt / .md 文本文件解析。')).toBeInTheDocument();
  });

  it('shows action buttons for each doc', async () => {
    renderDocsPage();
    await waitFor(() => {
      expect(screen.getAllByText('查看原文')).toHaveLength(2);
      expect(screen.getAllByText('重建索引')).toHaveLength(2);
      expect(screen.getAllByText('删除')).toHaveLength(2);
    });
  });

  it('shows chat page link', () => {
    renderDocsPage();
    expect(screen.getByRole('link', { name: '进入问答页面' })).toHaveAttribute('href', '/chat/kb_001');
  });

  it('delete button triggers mutation', async () => {
    vi.mocked(apiClient.apiDelete).mockResolvedValue({ deleted: true });
    const user = userEvent.setup();
    renderDocsPage();
    await waitFor(() => {
      expect(screen.getByText('OS笔记')).toBeInTheDocument();
    });
    const deleteButtons = screen.getAllByText('删除');
    await user.click(deleteButtons[0]);
    expect(apiClient.apiDelete).toHaveBeenCalledWith('/knowledge-bases/kb_001/documents/doc_1');
  });

  it('reindex button triggers mutation', async () => {
    vi.mocked(apiClient.apiPost).mockResolvedValue({ task_id: 'task_1' });
    const user = userEvent.setup();
    renderDocsPage();
    await waitFor(() => {
      expect(screen.getByText('OS笔记')).toBeInTheDocument();
    });
    const reindexButtons = screen.getAllByText('重建索引');
    await user.click(reindexButtons[0]);
    expect(apiClient.apiPost).toHaveBeenCalledWith('/knowledge-bases/kb_001/documents/doc_1/reindex');
  });

  it('shows task status when taskId is set', async () => {
    vi.mocked(apiClient.apiPost).mockResolvedValue({ task_id: 'task_123' });
    vi.mocked(apiClient.apiGet).mockImplementation(async (url: string) => {
      if (url.includes('/tasks/')) {
        return { task_id: 'task_123', type: 'document_index', status: 'processing', progress: 50, created_at: '', updated_at: '' };
      }
      return mockDocs;
    });
    const user = userEvent.setup();
    renderDocsPage();
    await waitFor(() => {
      expect(screen.getByText('OS笔记')).toBeInTheDocument();
    });
    const reindexButtons = screen.getAllByText('重建索引');
    await user.click(reindexButtons[0]);
    await waitFor(() => {
      expect(screen.getByText(/task_id: task_123/)).toBeInTheDocument();
    });
  });
});
