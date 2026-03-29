import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import App from './App';
import { useAuthStore } from '../stores/auth';

describe('App routes', () => {
  it('redirects to login when token is missing', () => {
    useAuthStore.getState().logout();
    render(
      <MemoryRouter initialEntries={['/kbs']}>
        <App />
      </MemoryRouter>,
    );
    expect(screen.getByText('登录 MemoBase')).toBeInTheDocument();
  });
});

