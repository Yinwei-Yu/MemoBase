import { beforeEach, describe, expect, it, vi } from 'vitest';

const {
  mockClient,
  createMock,
  isAxiosErrorMock,
} = vi.hoisted(() => {
  const client = {
    interceptors: {
      request: { use: vi.fn() },
    },
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
  };
  return {
    mockClient: client,
    createMock: vi.fn(() => client),
    isAxiosErrorMock: vi.fn((err: unknown) => Boolean((err as { __axios?: boolean })?.__axios)),
  };
});

vi.mock('axios', () => ({
  default: {
    create: createMock,
    isAxiosError: isAxiosErrorMock,
  },
  create: createMock,
  isAxiosError: isAxiosErrorMock,
}));

describe('api client edge cases', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  it('apiGet passes params correctly', async () => {
    const { apiGet } = await import('./client');
    mockClient.get.mockResolvedValueOnce({
      data: { data: { items: [] } },
    });

    await apiGet('/knowledge-bases', { page: 1, page_size: 20 });
    expect(mockClient.get).toHaveBeenCalledWith('/knowledge-bases', { params: { page: 1, page_size: 20 } });
  });

  it('apiPost sends payload correctly', async () => {
    const { apiPost } = await import('./client');
    mockClient.post.mockResolvedValueOnce({
      data: { data: { kb_id: 'kb_1' } },
    });

    const data = await apiPost('/knowledge-bases', { name: 'test' });
    expect(mockClient.post).toHaveBeenCalledWith('/knowledge-bases', { name: 'test' });
    expect(data).toEqual({ kb_id: 'kb_1' });
  });

  it('apiPatch sends payload correctly', async () => {
    const { apiPatch } = await import('./client');
    mockClient.patch.mockResolvedValueOnce({
      data: { data: { kb_id: 'kb_1', name: 'updated' } },
    });

    const data = await apiPatch('/knowledge-bases/kb_1', { name: 'updated' });
    expect(mockClient.patch).toHaveBeenCalledWith('/knowledge-bases/kb_1', { name: 'updated' });
    expect(data).toEqual({ kb_id: 'kb_1', name: 'updated' });
  });

  it('apiDelete sends delete request', async () => {
    const { apiDelete } = await import('./client');
    mockClient.delete.mockResolvedValueOnce({
      data: { data: { deleted: true } },
    });

    const data = await apiDelete('/knowledge-bases/kb_1');
    expect(mockClient.delete).toHaveBeenCalledWith('/knowledge-bases/kb_1');
    expect(data).toEqual({ deleted: true });
  });

  it('apiGet throws generic error for non-axios errors', async () => {
    const { apiGet } = await import('./client');
    mockClient.get.mockRejectedValueOnce(new Error('network error'));

    await expect(apiGet('/x')).rejects.toThrow('network error');
  });

  it('apiGet throws unknown error for non-error values', async () => {
    const { apiGet } = await import('./client');
    isAxiosErrorMock.mockReturnValueOnce(false);
    mockClient.get.mockRejectedValueOnce('string error');

    await expect(apiGet('/x')).rejects.toThrow('unknown api error');
  });

  it('apiGet throws axios message when no error body', async () => {
    const { apiGet } = await import('./client');
    isAxiosErrorMock.mockReturnValueOnce(true);
    mockClient.get.mockRejectedValueOnce({
      __axios: true,
      response: { data: null },
      message: 'Request failed with status code 500',
    });

    await expect(apiGet('/x')).rejects.toThrow('Request failed with status code 500');
  });

  it('apiPost throws error with code and message', async () => {
    const { apiPost } = await import('./client');
    isAxiosErrorMock.mockReturnValueOnce(true);
    mockClient.post.mockRejectedValueOnce({
      __axios: true,
      response: {
        data: {
          error: {
            code: 'KB_NOT_FOUND',
            message: 'knowledge base not found',
          },
        },
      },
      message: 'Request failed',
    });

    await expect(apiPost('/x', {})).rejects.toThrow('KB_NOT_FOUND: knowledge base not found');
  });
});
