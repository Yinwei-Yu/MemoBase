import React from 'react';

type Props = {
  children: React.ReactNode;
};

type State = {
  hasError: boolean;
  message: string;
};

export default class AppErrorBoundary extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, message: '' };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, message: error.message };
  }

  componentDidCatch(error: Error): void {
    console.error('ui_error_boundary', error);
  }

  render() {
    if (this.state.hasError) {
      return (
        <main className="page-shell">
          <section className="card">
            <h2>页面渲染异常</h2>
            <p className="muted">请刷新页面或重新登录后重试。</p>
            <pre>{this.state.message}</pre>
          </section>
        </main>
      );
    }
    return this.props.children;
  }
}
