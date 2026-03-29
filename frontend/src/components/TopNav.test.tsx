import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import TopNav from './TopNav';
import { useAuthStore } from '../stores/auth';

describe('TopNav', () => {
  it('renders user display name and logs out', () => {
    useAuthStore.getState().setAuth('token_1', {
      user_id: 'u_1',
      username: 'demo',
      display_name: 'Demo User',
    });

    render(
      <MemoryRouter>
        <TopNav />
      </MemoryRouter>,
    );

    expect(screen.getByText('Demo User')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '退出' }));
    expect(useAuthStore.getState().token).toBe('');
  });
});

