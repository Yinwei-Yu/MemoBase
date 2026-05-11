import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import AppErrorBoundary from './AppErrorBoundary';

function Thrower({ message = 'boom' }: { message?: string }): JSX.Element {
  throw new Error(message);
}

function SafeChild(): JSX.Element {
  return <div>safe content</div>;
}

describe('AppErrorBoundary edge cases', () => {
  it('renders children when no error', () => {
    render(
      <AppErrorBoundary>
        <SafeChild />
      </AppErrorBoundary>,
    );
    expect(screen.getByText('safe content')).toBeInTheDocument();
  });

  it('shows error message from thrown error', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(
      <AppErrorBoundary>
        <Thrower message="custom error message" />
      </AppErrorBoundary>,
    );
    expect(screen.getByText('custom error message')).toBeInTheDocument();
    spy.mockRestore();
  });

  it('shows refresh hint', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(
      <AppErrorBoundary>
        <Thrower />
      </AppErrorBoundary>,
    );
    expect(screen.getByText('请刷新页面或重新登录后重试。')).toBeInTheDocument();
    spy.mockRestore();
  });

  it('shows heading in fallback', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(
      <AppErrorBoundary>
        <Thrower />
      </AppErrorBoundary>,
    );
    expect(screen.getByText('页面渲染异常')).toBeInTheDocument();
    spy.mockRestore();
  });

  it('renders multiple safe children', () => {
    render(
      <AppErrorBoundary>
        <div>child 1</div>
        <div>child 2</div>
      </AppErrorBoundary>,
    );
    expect(screen.getByText('child 1')).toBeInTheDocument();
    expect(screen.getByText('child 2')).toBeInTheDocument();
  });

  it('handles error with empty message', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(
      <AppErrorBoundary>
        <Thrower message="" />
      </AppErrorBoundary>,
    );
    expect(screen.getByText('页面渲染异常')).toBeInTheDocument();
    spy.mockRestore();
  });

  it('logs error to console', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    render(
      <AppErrorBoundary>
        <Thrower message="test error" />
      </AppErrorBoundary>,
    );
    expect(spy).toHaveBeenCalledWith('ui_error_boundary', expect.any(Error));
    spy.mockRestore();
  });
});
