import { useQuery } from '@tanstack/react-query';
import { client } from '../lib/api/client';

type ReadyResponse = {
  data: {
    status: string;
    checks: Record<string, string>;
  };
};

export default function OpsPage() {
  const healthQuery = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      const resp = await client.get<ReadyResponse>('/readyz');
      return resp.data.data;
    },
    refetchInterval: 5000,
  });

  return (
    <section className="page-grid ops-grid">
      <header className="page-head">
        <div>
          <p className="eyebrow">Observability</p>
          <h1>系统健康度</h1>
          <p>实时检查核心依赖状态，快速定位异常服务并观察整体运行趋势。</p>
        </div>
      </header>
      <div className="card">
        <h2>系统状态</h2>
        {healthQuery.isLoading && <p className="system-tip">检测中...</p>}
        {healthQuery.isError && <div className="error-box">{(healthQuery.error as Error).message}</div>}
        {healthQuery.data && (
          <>
            <p className="system-tip">
              overall: <strong>{healthQuery.data.status}</strong>
            </p>
            <div className="list">
              {Object.entries(healthQuery.data.checks).map(([key, value]) => (
                <div key={key} className="list-item">
                  <h3>{key}</h3>
                  <span className={value === 'up' ? 'pill success' : 'pill danger'}>{value}</span>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    </section>
  );
}
