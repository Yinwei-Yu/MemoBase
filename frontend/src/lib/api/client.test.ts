import { beforeEach, describe, expect, it, vi } from 'vitest';

const {
  mockClient,
  requestUseMock,
  createMock,
  isAxiosErrorMock,
} = vi.hoisted(() => {
  const requestUse = vi.fn();
  const client = {
    interceptors: {
      request: {
        use: requestUse,
      },
    },
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  };
  return {
    mockClient: client,
    requestUseMock: requestUse,
    createMock: vi.fn(() => client),
    isAxiosErrorMock: vi.fn((err: unknown) => Boolean((err as { __axios?: boolean })?.__axios)),
  };
});

vi.mock('axios', () => {
  return {
    default: {
      create: createMock,
      isAxiosError: isAxiosErrorMock,
    },
    create: createMock,
    isAxiosError: isAxiosErrorMock,
  };
});

describe('api client helpers', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  it('injects bearer token in request interceptor', async () => {
    const { useAuthStore } = await import('../../stores/auth');
    useAuthStore.getState().setAuth('token_abc', {
      user_id: 'u_1',
      username: 'demo',
      display_name: 'Demo',
    });

    await import('./client');
    const interceptor = requestUseMock.mock.calls[0]?.[0] as ((cfg: { headers: Record<string, string> }) => {
      headers: Record<string, string>;
    }) | undefined;

    expect(interceptor).toBeTruthy();
    const cfg = interceptor!({ headers: {} });
    expect(cfg.headers.Authorization).toBe('Bearer token_abc');
  });

  it('apiGet unwraps response data', async () => {
    const { apiGet } = await import('./client');
    mockClient.get.mockResolvedValueOnce({
      data: {
        data: { id: 'x' },
      },
    });

    const data = await apiGet<{ id: string }>('/x');
    expect(data).toEqual({ id: 'x' });
  });

  it('apiPost maps backend error shape', async () => {
    const { apiPost } = await import('./client');
    mockClient.post.mockRejectedValueOnce({
      __axios: true,
      response: {
        data: {
          error: {
            code: 'VALIDATION_ERROR',
            message: 'invalid payload',
          },
        },
      },
      message: 'request failed',
    });

    await expect(apiPost('/x', { a: 1 })).rejects.toThrow('VALIDATION_ERROR: invalid payload');
  });
});

