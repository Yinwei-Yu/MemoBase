import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import OpsPage from './OpsPage';
import { client } from '../lib/api/client';

vi.mock('../lib/api/client', () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiUpload: vi.fn(),
  apiDelete: vi.fn(),
  apiPatch: vi.fn(),
  client: { get: vi.fn() },
}));

function renderOpsPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/ops']}>
        <OpsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const mockHealthData = {
  data: {
    data: {
      status: 'ready',
      checks: {
        db: 'up',
        qdrant: 'up',
        storage: 'up',
        model_gateway: 'up',
      },
    },
  },
};

describe('OpsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(client.get).mockResolvedValue(mockHealthData);
  });

  it('renders heading', () => {
    renderOpsPage();
    expect(screen.getByText('系统健康度')).toBeInTheDocument();
  });

  it('shows loading state', () => {
    vi.mocked(client.get).mockImplementation(() => new Promise(() => {}));
    renderOpsPage();
    expect(screen.getByText('检测中...')).toBeInTheDocument();
  });

  it('shows overall status', async () => {
    renderOpsPage();
    await waitFor(() => {
      expect(screen.getByText('ready')).toBeInTheDocument();
    });
  });

  it('shows all health checks', async () => {
    renderOpsPage();
    await waitFor(() => {
      expect(screen.getByText('db')).toBeInTheDocument();
      expect(screen.getByText('qdrant')).toBeInTheDocument();
      expect(screen.getByText('storage')).toBeInTheDocument();
      expect(screen.getByText('model_gateway')).toBeInTheDocument();
    });
  });

  it('shows up status pills', async () => {
    renderOpsPage();
    await waitFor(() => {
      const upPills = screen.getAllByText('up');
      expect(upPills.length).toBe(4);
    });
  });

  it('shows error on failure', async () => {
    vi.mocked(client.get).mockRejectedValue(new Error('ECONNREFUSED'));
    renderOpsPage();
    await waitFor(() => {
      expect(screen.getByText(/ECONNREFUSED/)).toBeInTheDocument();
    });
  });

  it('shows degraded status when dependency is down', async () => {
    vi.mocked(client.get).mockResolvedValue({
      data: {
        data: {
          status: 'not_ready',
          checks: {
            db: 'up',
            qdrant: 'down',
            storage: 'up',
            model_gateway: 'up',
          },
        },
      },
    });
    renderOpsPage();
    await waitFor(() => {
      expect(screen.getByText('not_ready')).toBeInTheDocument();
      expect(screen.getByText('down')).toBeInTheDocument();
    });
  });
});
