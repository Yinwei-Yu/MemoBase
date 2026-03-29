import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import AppErrorBoundary from './AppErrorBoundary';

function Thrower(): JSX.Element {
  throw new Error('boom');
}

describe('AppErrorBoundary', () => {
  it('renders fallback ui when child throws', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(
      <AppErrorBoundary>
        <Thrower />
      </AppErrorBoundary>,
    );
    expect(screen.getByText('页面渲染异常')).toBeInTheDocument();
    expect(screen.getByText('boom')).toBeInTheDocument();
    spy.mockRestore();
  });
});
