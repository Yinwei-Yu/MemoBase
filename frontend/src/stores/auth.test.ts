import { describe, expect, it, vi } from 'vitest';

type AuthModule = typeof import('./auth');

async function loadAuthModule(): Promise<AuthModule> {
  vi.resetModules();
  return import('./auth');
}

describe('auth store', () => {
  it('loads initial auth from localStorage', async () => {
    localStorage.setItem('memo_token', 'token_1');
    localStorage.setItem(
      'memo_user',
      JSON.stringify({
        user_id: 'u_1',
        username: 'demo',
        display_name: 'Demo',
      }),
    );

    const { useAuthStore } = await loadAuthModule();
    const state = useAuthStore.getState();
    expect(state.token).toBe('token_1');
    expect(state.user?.user_id).toBe('u_1');
  });

  it('drops invalid persisted user JSON', async () => {
    localStorage.setItem('memo_user', '{bad-json');

    const { useAuthStore } = await loadAuthModule();
    const state = useAuthStore.getState();
    expect(state.user).toBeNull();
    expect(localStorage.getItem('memo_user')).toBeNull();
  });

  it('setAuth and logout update state and localStorage', async () => {
    const { useAuthStore } = await loadAuthModule();
    useAuthStore.getState().setAuth('token_2', {
      user_id: 'u_2',
      username: 'demo2',
      display_name: 'Demo 2',
    });

    expect(useAuthStore.getState().token).toBe('token_2');
    expect(localStorage.getItem('memo_token')).toBe('token_2');

    useAuthStore.getState().logout();
    expect(useAuthStore.getState().token).toBe('');
    expect(useAuthStore.getState().user).toBeNull();
    expect(localStorage.getItem('memo_token')).toBeNull();
    expect(localStorage.getItem('memo_user')).toBeNull();
  });
});

