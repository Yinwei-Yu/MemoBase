import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiDelete, apiGet, apiPatch, apiPost } from '../lib/api/client';
import type { Memory } from '../lib/types/api';

const TYPE_LABELS: Record<string, string> = {
  long_term: '长期记忆',
  fact: '事实',
  preference: '偏好',
  user_profile: '用户画像',
};

const TYPE_COLORS: Record<string, string> = {
  long_term: '#6366f1',
  fact: '#10b981',
  preference: '#f59e0b',
  user_profile: '#8b5cf6',
};

export default function MemoriesPage() {
  const queryClient = useQueryClient();
  const [typeFilter, setTypeFilter] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editSummary, setEditSummary] = useState('');
  const [editImportance, setEditImportance] = useState(0.5);

  const memoriesQuery = useQuery({
    queryKey: ['memories', typeFilter],
    queryFn: () => apiGet<{ memories: Memory[] }>('/memories', {
      ...(typeFilter ? { type: typeFilter } : {}),
      limit: 200,
    }),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, summary, importance }: { id: string; summary: string; importance: number }) =>
      apiPatch<Memory, { summary: string; importance: number }>(`/memories/${id}`, { summary, importance }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['memories'] });
      setEditingId(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiDelete(`/memories/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['memories'] });
    },
  });

  const consolidateMutation = useMutation({
    mutationFn: () => apiPost('/memories/consolidate'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['memories'] });
    },
  });

  function startEdit(mem: Memory) {
    setEditingId(mem.memory_id);
    setEditSummary(mem.summary);
    setEditImportance(mem.importance);
  }

  const memories = memoriesQuery.data?.memories ?? [];

  return (
    <section className="page-grid">
      <header className="page-head">
        <div>
          <p className="eyebrow">Memory</p>
          <h1>记忆管理</h1>
          <p>查看和管理 AI 从对话中提取的长期记忆。</p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="button-secondary"
            style={{ padding: '0.4rem 0.6rem' }}
          >
            <option value="">全部类型</option>
            <option value="long_term">长期记忆</option>
            <option value="fact">事实</option>
            <option value="preference">偏好</option>
            <option value="user_profile">用户画像</option>
          </select>
          <button
            type="button"
            className="button-like"
            onClick={() => consolidateMutation.mutate()}
            disabled={consolidateMutation.isPending}
          >
            {consolidateMutation.isPending ? '整理中...' : '整理记忆'}
          </button>
        </div>
      </header>

      {consolidateMutation.isSuccess && (
        <div className="status-banner healthy" style={{ marginBottom: '0.75rem' }}>
          记忆整理完成
        </div>
      )}
      {consolidateMutation.isError && (
        <div className="error-box" style={{ marginBottom: '0.75rem' }}>
          整理失败: {(consolidateMutation.error as Error).message}
        </div>
      )}

      {memoriesQuery.isLoading && <p className="system-tip">加载中...</p>}
      {memoriesQuery.isError && (
        <div className="error-box">{(memoriesQuery.error as Error).message}</div>
      )}

      {memories.length === 0 && !memoriesQuery.isLoading && (
        <div className="card">
          <p className="muted">暂无记忆数据。开始对话后 AI 会自动提取记忆。</p>
        </div>
      )}

      {memories.map((mem) => (
        <div key={mem.memory_id} className="card">
          {editingId === mem.memory_id ? (
            <div className="form-grid">
              <label>
                <span>摘要</span>
                <textarea
                  value={editSummary}
                  onChange={(e) => setEditSummary(e.target.value)}
                  rows={3}
                  maxLength={500}
                />
              </label>
              <label>
                <span>重要度 ({editImportance.toFixed(1)})</span>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.1"
                  value={editImportance}
                  onChange={(e) => setEditImportance(parseFloat(e.target.value))}
                />
              </label>
              <div className="form-actions">
                <button
                  type="button"
                  className="button-like"
                  onClick={() => updateMutation.mutate({ id: mem.memory_id, summary: editSummary, importance: editImportance })}
                  disabled={updateMutation.isPending}
                >
                  {updateMutation.isPending ? '保存中...' : '保存'}
                </button>
                <button type="button" className="button-secondary" onClick={() => setEditingId(null)}>
                  取消
                </button>
              </div>
              {updateMutation.isError && (
                <div className="error-box">{(updateMutation.error as Error).message}</div>
              )}
            </div>
          ) : (
            <div className="list-item" style={{ alignItems: 'flex-start' }}>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', marginBottom: '0.25rem' }}>
                  <span
                    className="pill"
                    style={{ background: TYPE_COLORS[mem.type] ?? '#64748b', color: '#fff', fontSize: 'var(--text-xs)' }}
                  >
                    {TYPE_LABELS[mem.type] ?? mem.type}
                  </span>
                  <span className="muted" style={{ fontSize: 'var(--text-xs)' }}>
                    重要度: {mem.importance.toFixed(1)}
                  </span>
                  <span className="muted" style={{ fontSize: 'var(--text-xs)' }}>
                    访问: {mem.access_count}次
                  </span>
                </div>
                <p style={{ margin: 0, lineHeight: 1.6 }}>{mem.summary}</p>
                <p className="muted" style={{ fontSize: 'var(--text-xs)', margin: '0.25rem 0 0' }}>
                  创建: {new Date(mem.created_at).toLocaleString()}
                  {mem.last_accessed_at && ` · 最后访问: ${new Date(mem.last_accessed_at).toLocaleString()}`}
                </p>
              </div>
              <div style={{ display: 'flex', gap: '0.5rem', flexShrink: 0 }}>
                <button type="button" className="button-secondary" onClick={() => startEdit(mem)}>
                  编辑
                </button>
                <button
                  type="button"
                  className="button-secondary"
                  onClick={() => {
                    if (confirm('确定删除这条记忆?')) deleteMutation.mutate(mem.memory_id);
                  }}
                >
                  删除
                </button>
              </div>
            </div>
          )}
        </div>
      ))}
    </section>
  );
}
