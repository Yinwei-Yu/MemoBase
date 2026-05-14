import { useQuery } from '@tanstack/react-query';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { client } from '../lib/api/client';

type ReadyResponse = {
  data: {
    status: string;
    checks: Record<string, string>;
  };
};

type MetricsSnapshot = {
  in_flight: number;
  total_requests: number;
  by_route: Array<{
    method: string;
    route: string;
    count: number;
    avg_seconds: number;
  }>;
  status_breakdown: {
    "2xx": number;
    "4xx": number;
    "5xx": number;
  };
};

type PrometheusSeries = {
  label: string;
  values: [number, string][];
};

type PrometheusQueryResponse = {
  data: {
    series: PrometheusSeries[];
  };
};

// Colors for chart lines
const COLORS = ['#6366f1', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6', '#06b6d4'];

function fmtMs(seconds: number): string {
  if (seconds < 0.001) return '<1ms';
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
  return `${seconds.toFixed(2)}s`;
}

function fmtTime(ts: number): string {
  return new Date(ts * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
}

// Build time range params for Prometheus query_range
function timeRange() {
  const end = Math.floor(Date.now() / 1000);
  const start = end - 3600; // last 1 hour
  return { start: String(start), end: String(end), step: '60' };
}

function usePrometheusQuery(query: string) {
  return useQuery({
    queryKey: ['prometheus', query],
    queryFn: async () => {
      const params = { query, ...timeRange() };
      const resp = await client.get<PrometheusQueryResponse>('/metrics/prometheus', { params });
      return resp.data.data.series;
    },
    refetchInterval: 15000,
  });
}

// Transform Prometheus series into recharts-compatible data
function toChartData(series: PrometheusSeries[]) {
  if (!series.length) return [];

  // Collect all timestamps from first series
  const timestamps = series[0].values.map(([ts]) => ts);

  return timestamps.map((ts, i) => {
    const point: Record<string, number | string> = { time: fmtTime(ts) };
    series.forEach((s) => {
      const val = s.values[i]?.[1];
      point[s.label] = val != null ? parseFloat(val) : 0;
    });
    return point;
  });
}

function PrometheusChart({ title, query, unit, colorIndex = 0 }: {
  title: string;
  query: string;
  unit?: string;
  colorIndex?: number;
}) {
  const { data: series, isLoading, isError, error } = usePrometheusQuery(query);

  const chartData = series ? toChartData(series) : [];
  const labels = series?.map((s) => s.label) ?? [];

  return (
    <div className="card prometheus-chart-card">
      <h2>{title}</h2>
      {isLoading && <p className="system-tip">加载中...</p>}
      {isError && <div className="error-box">{(error as Error).message}</div>}
      {!isLoading && !isError && chartData.length === 0 && (
        <p className="system-tip">暂无数据</p>
      )}
      {chartData.length > 0 && (
        <div className="chart-container">
          <ResponsiveContainer width="100%" height={280}>
            <LineChart data={chartData} margin={{ top: 8, right: 16, left: 0, bottom: 8 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-primary)" />
              <XAxis
                dataKey="time"
                tick={{ fontSize: 11, fill: 'var(--text-tertiary)' }}
                tickLine={false}
                axisLine={{ stroke: 'var(--border-primary)' }}
              />
              <YAxis
                tick={{ fontSize: 11, fill: 'var(--text-tertiary)' }}
                tickLine={false}
                axisLine={{ stroke: 'var(--border-primary)' }}
                width={60}
                tickFormatter={unit === 'reqps' ? (v) => `${v}/s` : undefined}
              />
              <Tooltip
                contentStyle={{
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border-primary)',
                  borderRadius: 'var(--radius-md)',
                  fontSize: 'var(--text-xs)',
                }}
              />
              <Legend
                wrapperStyle={{ fontSize: 'var(--text-xs)', paddingTop: 8 }}
              />
              {labels.map((label, i) => (
                <Line
                  key={label}
                  type="monotone"
                  dataKey={label}
                  stroke={COLORS[(colorIndex + i) % COLORS.length]}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4 }}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

export default function OpsPage() {
  const healthQuery = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      const resp = await client.get<ReadyResponse>('/readyz');
      return resp.data.data;
    },
    refetchInterval: 5000,
  });

  const metricsQuery = useQuery({
    queryKey: ['metrics-summary'],
    queryFn: async () => {
      const resp = await client.get<{ data: MetricsSnapshot }>('/metrics/summary');
      return resp.data.data;
    },
    refetchInterval: 10000,
  });

  const metrics = metricsQuery.data;

  return (
    <section className="page-grid ops-grid">
      <header className="page-head">
        <div>
          <p className="eyebrow">Observability</p>
          <h1>系统健康度</h1>
          <p>实时检查核心依赖状态，快速定位异常服务并观察整体运行趋势。</p>
        </div>
      </header>

      {/* ---- Health Status ---- */}
      <div className="card">
        <h2>系统状态</h2>
        {healthQuery.isLoading && <p className="system-tip">检测中...</p>}
        {healthQuery.isError && (
          <div className="error-box">{(healthQuery.error as Error).message}</div>
        )}
        {healthQuery.data && (
          <>
            <div
              className={`status-banner ${
                healthQuery.data.status === 'ok' || healthQuery.data.status === 'ready'
                  ? 'healthy'
                  : 'unhealthy'
              }`}
            >
              <span
                className={`status-dot ${
                  healthQuery.data.status === 'ok' || healthQuery.data.status === 'ready'
                    ? 'up'
                    : 'down'
                }`}
              />
              整体状态:{' '}
              {healthQuery.data.status === 'ok' || healthQuery.data.status === 'ready'
                ? '正常运行'
                : '异常'}
            </div>
            <div className="list">
              {Object.entries(healthQuery.data.checks).map(([key, value]) => (
                <div key={key} className="list-item">
                  <h3>{key}</h3>
                  <span className={`pill ${value === 'up' ? 'success' : 'danger'}`}>
                    <span className={`status-dot ${value === 'up' ? 'up' : 'down'}`} />
                    {value}
                  </span>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      {/* ---- HTTP Metrics ---- */}
      <div className="card">
        <h2>HTTP 指标</h2>
        {metricsQuery.isLoading && <p className="system-tip">加载中...</p>}
        {metricsQuery.isError && (
          <div className="error-box">{(metricsQuery.error as Error).message}</div>
        )}
        {metrics && (
          <>
            <div className="metrics-grid">
              <div className="metric-card">
                <span className="metric-value">{metrics.total_requests}</span>
                <span className="metric-label">总请求数</span>
              </div>
              <div className="metric-card">
                <span className="metric-value">{metrics.in_flight}</span>
                <span className="metric-label">正在处理</span>
              </div>
            </div>

            {/* Status breakdown bar */}
            <div className="status-breakdown">
              <span className="metric-subtitle">状态分布</span>
              <div className="status-bar-track">
                {metrics.total_requests > 0 ? (
                  <>
                    <div
                      className="status-bar-segment s2xx"
                      style={{
                        width: `${(metrics.status_breakdown['2xx'] / metrics.total_requests) * 100}%`,
                      }}
                      title={`2xx: ${metrics.status_breakdown['2xx']}`}
                    />
                    <div
                      className="status-bar-segment s4xx"
                      style={{
                        width: `${(metrics.status_breakdown['4xx'] / metrics.total_requests) * 100}%`,
                      }}
                      title={`4xx: ${metrics.status_breakdown['4xx']}`}
                    />
                    <div
                      className="status-bar-segment s5xx"
                      style={{
                        width: `${(metrics.status_breakdown['5xx'] / metrics.total_requests) * 100}%`,
                      }}
                      title={`5xx: ${metrics.status_breakdown['5xx']}`}
                    />
                  </>
                ) : (
                  <div className="status-bar-segment s2xx" style={{ width: '100%' }} />
                )}
              </div>
              <div className="status-bar-legend">
                <span className="legend-item">
                  <span className="legend-dot s2xx" /> 2xx: {metrics.status_breakdown['2xx']}
                </span>
                <span className="legend-item">
                  <span className="legend-dot s4xx" /> 4xx: {metrics.status_breakdown['4xx']}
                </span>
                <span className="legend-item">
                  <span className="legend-dot s5xx" /> 5xx: {metrics.status_breakdown['5xx']}
                </span>
              </div>
            </div>

            {/* Per-route metrics */}
            {metrics.by_route.length > 0 && (
              <div className="route-metrics">
                <span className="metric-subtitle">路由详情</span>
                <div className="route-table">
                  <div className="route-table-header">
                    <span>路由</span>
                    <span>方法</span>
                    <span>请求数</span>
                    <span>平均延迟</span>
                  </div>
                  {metrics.by_route.map((r) => (
                    <div key={`${r.method}-${r.route}`} className="route-table-row">
                      <span className="route-path" title={r.route}>
                        {r.route}
                      </span>
                      <span className="route-method">{r.method}</span>
                      <span>{r.count}</span>
                      <span>{fmtMs(r.avg_seconds)}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* ---- Prometheus Time-Series Charts ---- */}
      <PrometheusChart
        title="请求速率 (req/s)"
        query="sum(rate(memobase_http_requests_total[1m])) by (status)"
        unit="reqps"
        colorIndex={0}
      />

      <PrometheusChart
        title="请求延迟 (avg)"
        query="rate(memobase_http_request_duration_seconds_sum[1m]) / rate(memobase_http_request_duration_seconds_count[1m])"
        unit="seconds"
        colorIndex={2}
      />
    </section>
  );
}
