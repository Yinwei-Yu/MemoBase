import { create } from 'zustand';
import type { User } from '../lib/types/api';

type AuthState = {
  token: string;
  user: User | null;
  setAuth: (token: string, user: User) => void;
  logout: () => void;
};

const initialToken = localStorage.getItem('memo_token') ?? '';
const initialUser = localStorage.getItem('memo_user');

function parseUser(raw: string | null): User | null {
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as User;
  } catch {
    localStorage.removeItem('memo_user');
    return null;
  }
}

export const useAuthStore = create<AuthState>((set) => ({
  token: initialToken,
  user: parseUser(initialUser),
  setAuth: (token, user) => {
    localStorage.setItem('memo_token', token);
    localStorage.setItem('memo_user', JSON.stringify(user));
    set({ token, user });
  },
  logout: () => {
    localStorage.removeItem('memo_token');
    localStorage.removeItem('memo_user');
    set({ token: '', user: null });
  },
}));
