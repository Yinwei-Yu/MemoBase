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

const mockMetricsData = {
  data: {
    data: {
      in_flight: 2,
      total_requests: 100,
      by_route: [
        { method: 'GET', route: '/api/v1/healthz', count: 50, avg_seconds: 0.001 },
      ],
      status_breakdown: {
        '2xx': 95,
        '4xx': 3,
        '5xx': 2,
      },
    },
  },
};

const mockPrometheusData = {
  data: {
    data: {
      series: [],
    },
  },
};

describe('OpsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(client.get).mockImplementation((url: string) => {
      if (url === '/readyz') return Promise.resolve(mockHealthData);
      if (url === '/metrics/summary') return Promise.resolve(mockMetricsData);
      if (url === '/metrics/prometheus') return Promise.resolve(mockPrometheusData);
      return Promise.resolve(mockHealthData);
    });
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
      expect(screen.getByText(/正常运行/)).toBeInTheDocument();
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
    vi.mocked(client.get).mockImplementation((url: string) => {
      if (url === '/readyz') {
        return Promise.resolve({
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
      }
      if (url === '/metrics/summary') return Promise.resolve(mockMetricsData);
      if (url === '/metrics/prometheus') return Promise.resolve(mockPrometheusData);
      return Promise.resolve(mockHealthData);
    });
    const { container } = renderOpsPage();
    await waitFor(() => {
      const banner = container.querySelector('.status-banner.unhealthy');
      expect(banner).not.toBeNull();
      expect(screen.getByText('down')).toBeInTheDocument();
    });
  });
});
