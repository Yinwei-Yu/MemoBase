import { useQuery } from '@tanstack/react-query';
import { client } from '../lib/api/client';
import { useState } from 'react';

type ReadyResponse = {
  data: {
    status: string;
    checks: Record<string, string>;
  };
};

const GRAFANA_BASE = import.meta.env.VITE_GRAFANA_URL ?? 'http://localhost:3000';
const GRAFANA_DASHBOARD_UID = 'memobase-overview';

type TabKey = 'health' | 'grafana' | 'metrics';

export default function OpsPage() {
  const [activeTab, setActiveTab] = useState<TabKey>('health');

  const healthQuery = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      const resp = await client.get<ReadyResponse>('/readyz');
      return resp.data.data;
    },
    refetchInterval: 5000,
  });

  const metricsQuery = useQuery({
    queryKey: ['metrics'],
    queryFn: async () => {
      const resp = await client.get('/metrics', { responseType: 'text' });
      return parsePrometheusMetrics(resp.data as string);
    },
    refetchInterval: 10000,
  });

  return (
    <section className="page-grid ops-grid">
      <header className="page-head">
        <div>
          <p className="eyebrow">Observability</p>
          <h1>系统监控</h1>
          <p>实时查看系统健康状态、性能指标和 Grafana 监控面板。</p>
        </div>
      </header>

      <div className="ops-tabs">
        <button
          className={`tab-btn ${activeTab === 'health' ? 'active' : ''}`}
          onClick={() => setActiveTab('health')}
        >
          健康状态
        </button>
        <button
          className={`tab-btn ${activeTab === 'metrics' ? 'active' : ''}`}
          onClick={() => setActiveTab('metrics')}
        >
          实时指标
        </button>
        <button
          className={`tab-btn ${activeTab === 'grafana' ? 'active' : ''}`}
          onClick={() => setActiveTab('grafana')}
        >
          Grafana 面板
        </button>
      </div>

      {activeTab === 'health' && (
        <div className="card">
          <h2>依赖状态</h2>
          {healthQuery.isLoading && <p className="system-tip">检测中...</p>}
          {healthQuery.isError && <div className="error-box">{(healthQuery.error as Error).message}</div>}
          {healthQuery.data && (
            <>
              <p className="system-tip">
                整体状态: <strong className={healthQuery.data.status === 'ready' ? 'status-ok' : 'status-err'}>{healthQuery.data.status}</strong>
              </p>
              <div className="status-grid">
                {Object.entries(healthQuery.data.checks).map(([key, value]) => (
                  <div key={key} className="status-card">
                    <div className="status-icon">{value === 'up' ? '✓' : '✗'}</div>
                    <div className="status-info">
                      <h3>{getServiceName(key)}</h3>
                      <span className={value === 'up' ? 'pill success' : 'pill danger'}>{value}</span>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      {activeTab === 'metrics' && (
        <div className="card">
          <h2>实时指标</h2>
          {metricsQuery.isLoading && <p className="system-tip">加载中...</p>}
          {metricsQuery.isError && <div className="error-box">{(metricsQuery.error as Error).message}</div>}
          {metricsQuery.data && (
            <div className="metrics-grid">
              <MetricCard
                title="服务状态"
                value={metricsQuery.data.memobase_up === 1 ? '运行中' : '已停止'}
                status={metricsQuery.data.memobase_up === 1 ? 'ok' : 'err'}
              />
              <MetricCard
                title="在途请求"
                value={String(metricsQuery.data.memobase_http_requests_in_flight ?? 0)}
                status="info"
              />
              <MetricCard
                title="请求总数"
                value={formatNumber(metricsQuery.data.memobase_http_requests_total ?? 0)}
                status="info"
              />
              <MetricCard
                title="平均延迟"
                value={formatDuration(metricsQuery.data.avg_duration_seconds ?? 0)}
                status={metricsQuery.data.avg_duration_seconds > 1 ? 'warn' : 'ok'}
              />
            </div>
          )}
        </div>
      )}

      {activeTab === 'grafana' && (
        <div className="card grafana-card">
          <div className="grafana-header">
            <h2>Grafana 监控面板</h2>
            <a href={`${GRAFANA_BASE}/d/${GRAFANA_DASHBOARD_UID}`} target="_blank" rel="noopener noreferrer" className="ghost-btn">
              在新窗口打开 ↗
            </a>
          </div>
          <div className="grafana-iframe-container">
            <iframe
              src={`${GRAFANA_BASE}/d-solo/${GRAFANA_DASHBOARD_UID}?orgId=1&theme=light&panelId=1`}
              width="100%"
              height="200"
              frameBorder="0"
              title="Service Status"
            />
            <iframe
              src={`${GRAFANA_BASE}/d-solo/${GRAFANA_DASHBOARD_UID}?orgId=1&theme=light&panelId=3`}
              width="100%"
              height="300"
              frameBorder="0"
              title="Request Rate"
            />
            <iframe
              src={`${GRAFANA_BASE}/d-solo/${GRAFANA_DASHBOARD_UID}?orgId=1&theme=light&panelId=4`}
              width="100%"
              height="300"
              frameBorder="0"
              title="Request Duration"
            />
            <iframe
              src={`${GRAFANA_BASE}/d-solo/${GRAFANA_DASHBOARD_UID}?orgId=1&theme=light&panelId=5`}
              width="100%"
              height="300"
              frameBorder="0"
              title="Memory Usage"
            />
          </div>
        </div>
      )}
    </section>
  );
}

function getServiceName(key: string): string {
  const names: Record<string, string> = {
    db: 'PostgreSQL',
    qdrant: 'Qdrant 向量库',
    model_gateway: 'Ollama 模型网关',
    storage: '文件存储',
  };
  return names[key] ?? key;
}

function MetricCard({ title, value, status }: { title: string; value: string; status: 'ok' | 'warn' | 'err' | 'info' }) {
  return (
    <div className={`metric-card metric-${status}`}>
      <div className="metric-title">{title}</div>
      <div className="metric-value">{value}</div>
    </div>
  );
}

interface PrometheusMetrics {
  memobase_up: number;
  memobase_http_requests_in_flight: number;
  memobase_http_requests_total: number;
  avg_duration_seconds: number;
}

function parsePrometheusMetrics(text: string): PrometheusMetrics {
  const lines = text.split('\n');
  const metrics: PrometheusMetrics = {
    memobase_up: 0,
    memobase_http_requests_in_flight: 0,
    memobase_http_requests_total: 0,
    avg_duration_seconds: 0,
  };

  let totalRequests = 0;
  let totalDurationSum = 0;
  let totalDurationCount = 0;

  for (const line of lines) {
    if (line.startsWith('#') || line.trim() === '') continue;

    const match = line.match(/^([a-zA-Z_][a-zA-Z0-9_]*)(?:\{[^}]*\})?\s+(.+)$/);
    if (!match) continue;

    const [, name, valueStr] = match;
    const value = parseFloat(valueStr);

    switch (name) {
      case 'memobase_up':
        metrics.memobase_up = value;
        break;
      case 'memobase_http_requests_in_flight':
        metrics.memobase_http_requests_in_flight = value;
        break;
      case 'memobase_http_requests_total':
        totalRequests += value;
        break;
      case 'memobase_http_request_duration_seconds_sum':
        totalDurationSum += value;
        break;
      case 'memobase_http_request_duration_seconds_count':
        totalDurationCount += value;
        break;
    }
  }

  metrics.memobase_http_requests_total = totalRequests;
  metrics.avg_duration_seconds = totalDurationCount > 0 ? totalDurationSum / totalDurationCount : 0;

  return metrics;
}

function formatNumber(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
  return String(Math.round(n));
}

function formatDuration(seconds: number): string {
  if (seconds < 0.001) return (seconds * 1000000).toFixed(0) + 'μs';
  if (seconds < 1) return (seconds * 1000).toFixed(0) + 'ms';
  return seconds.toFixed(2) + 's';
}
