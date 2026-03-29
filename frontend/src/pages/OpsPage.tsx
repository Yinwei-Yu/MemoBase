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
    <section className="page-grid">
      <div className="card">
        <h2>系统状态</h2>
        {healthQuery.isLoading && <p>检测中...</p>}
        {healthQuery.isError && <div className="error-box">{(healthQuery.error as Error).message}</div>}
        {healthQuery.data && (
          <>
            <p>
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
