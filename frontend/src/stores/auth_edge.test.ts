import { describe, expect, it, vi } from 'vitest';

type AuthModule = typeof import('./auth');

async function loadAuthModule(): Promise<AuthModule> {
  vi.resetModules();
  return import('./auth');
}

describe('auth store edge cases', () => {
  it('handles empty localStorage', async () => {
    localStorage.clear();
    const { useAuthStore } = await loadAuthModule();
    const state = useAuthStore.getState();
    expect(state.token).toBe('');
    expect(state.user).toBeNull();
  });

  it('handles null memo_user in localStorage', async () => {
    localStorage.removeItem('memo_user');
    localStorage.setItem('memo_token', 'some-token');
    const { useAuthStore } = await loadAuthModule();
    expect(useAuthStore.getState().token).toBe('some-token');
    expect(useAuthStore.getState().user).toBeNull();
  });

  it('handles empty string memo_user', async () => {
    localStorage.setItem('memo_user', '');
    const { useAuthStore } = await loadAuthModule();
    expect(useAuthStore.getState().user).toBeNull();
  });

  it('setAuth persists to localStorage correctly', async () => {
    const { useAuthStore } = await loadAuthModule();
    const user = { user_id: 'u_test', username: 'test', display_name: 'Test User' };
    useAuthStore.getState().setAuth('new-token', user);

    expect(localStorage.getItem('memo_token')).toBe('new-token');
    expect(JSON.parse(localStorage.getItem('memo_user')!)).toEqual(user);
  });

  it('logout clears all auth state', async () => {
    const { useAuthStore } = await loadAuthModule();
    useAuthStore.getState().setAuth('token', {
      user_id: 'u_1',
      username: 'user',
      display_name: 'User',
    });
    useAuthStore.getState().logout();

    expect(useAuthStore.getState().token).toBe('');
    expect(useAuthStore.getState().user).toBeNull();
    expect(localStorage.getItem('memo_token')).toBeNull();
    expect(localStorage.getItem('memo_user')).toBeNull();
  });

  it('multiple setAuth calls update state', async () => {
    const { useAuthStore } = await loadAuthModule();
    useAuthStore.getState().setAuth('token1', {
      user_id: 'u_1',
      username: 'user1',
      display_name: 'User 1',
    });
    expect(useAuthStore.getState().token).toBe('token1');

    useAuthStore.getState().setAuth('token2', {
      user_id: 'u_2',
      username: 'user2',
      display_name: 'User 2',
    });
    expect(useAuthStore.getState().token).toBe('token2');
    expect(useAuthStore.getState().user?.user_id).toBe('u_2');
  });

  it('handles malformed JSON in memo_user gracefully', async () => {
    localStorage.setItem('memo_user', '{"incomplete');
    const { useAuthStore } = await loadAuthModule();
    expect(useAuthStore.getState().user).toBeNull();
    // Should clean up bad data
    expect(localStorage.getItem('memo_user')).toBeNull();
  });

  it('handles memo_token with empty value', async () => {
    localStorage.setItem('memo_token', '');
    const { useAuthStore } = await loadAuthModule();
    expect(useAuthStore.getState().token).toBe('');
  });
});
