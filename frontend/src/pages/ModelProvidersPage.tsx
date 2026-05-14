import { FormEvent, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiDelete, apiGet, apiPost, apiPatch } from '../lib/api/client';
import type { ModelProvider, ProviderTestResult } from '../lib/types/api';

type FormState = {
  name: string;
  api_base_url: string;
  api_key: string;
  default_model: string;
  embedding_model: string;
};

const emptyForm: FormState = {
  name: '',
  api_base_url: '',
  api_key: '',
  default_model: '',
  embedding_model: '',
};

export default function ModelProvidersPage() {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<FormState>(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{ id: string; result: ProviderTestResult } | null>(null);
  const [showForm, setShowForm] = useState(false);

  const providersQuery = useQuery({
    queryKey: ['model-providers'],
    queryFn: () => apiGet<ModelProvider[]>('/model-providers'),
  });

  const createMutation = useMutation({
    mutationFn: (payload: FormState) =>
      apiPost<ModelProvider, FormState>('/model-providers', payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-providers'] });
      setForm(emptyForm);
      setShowForm(false);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, ...payload }: { id: string } & Partial<FormState & { is_default: boolean }>) =>
      apiPatch<ModelProvider, typeof payload>(`/model-providers/${id}`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-providers'] });
      setEditingId(null);
      setForm(emptyForm);
      setShowForm(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiDelete(`/model-providers/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-providers'] });
    },
  });

  const testMutation = useMutation({
    mutationFn: (id: string) => apiPost<ProviderTestResult>(`/model-providers/${id}/test`),
    onSuccess: (result, id) => {
      setTestResult({ id, result });
    },
    onError: (_err, id) => {
      setTestResult({
        id,
        result: { success: false, error: '请求失败', latency_ms: 0, model: '' },
      });
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (editingId) {
      updateMutation.mutate({ id: editingId, ...form });
    } else {
      createMutation.mutate(form);
    }
  }

  function startEdit(provider: ModelProvider) {
    setEditingId(provider.provider_id);
    setForm({
      name: provider.name,
      api_base_url: provider.api_base_url,
      api_key: '',
      default_model: provider.default_model,
      embedding_model: provider.embedding_model || '',
    });
    setShowForm(true);
  }

  function cancelForm() {
    setEditingId(null);
    setForm(emptyForm);
    setShowForm(false);
  }

  function setDefault(id: string) {
    updateMutation.mutate({ id, is_default: true });
  }

  const providers = providersQuery.data ?? [];

  return (
    <section className="page-grid">
      <header className="page-head">
        <div>
          <p className="eyebrow">Settings</p>
          <h1>模型提供商</h1>
          <p>配置第三方 AI 模型服务，用于知识问答和对话。</p>
        </div>
        {!showForm && (
          <button type="button" className="button-like" onClick={() => { setEditingId(null); setForm(emptyForm); setShowForm(true); }}>
            添加提供商
          </button>
        )}
      </header>

      {/* ---- Add / Edit Form ---- */}
      {showForm && (
        <div className="card">
          <h2>{editingId ? '编辑提供商' : '添加提供商'}</h2>
          <form onSubmit={onSubmit} className="form-grid">
            <label>
              <span>名称</span>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                placeholder="例如: DeepSeek / OpenAI"
                required
                maxLength={64}
              />
            </label>
            <label>
              <span>API Base URL</span>
              <input
                type="url"
                value={form.api_base_url}
                onChange={(e) => setForm((f) => ({ ...f, api_base_url: e.target.value }))}
                placeholder="https://api.deepseek.com"
                required
              />
            </label>
            <label>
              <span>API Key</span>
              <input
                type="password"
                value={form.api_key}
                onChange={(e) => setForm((f) => ({ ...f, api_key: e.target.value }))}
                placeholder={editingId ? '留空则不修改' : 'sk-...'}
                {...(editingId ? {} : { required: true })}
              />
            </label>
            <label>
              <span>默认模型</span>
              <input
                type="text"
                value={form.default_model}
                onChange={(e) => setForm((f) => ({ ...f, default_model: e.target.value }))}
                placeholder="deepseek-chat"
                required
              />
            </label>
            <label>
              <span>Embedding 模型 <span className="muted">(可选)</span></span>
              <input
                type="text"
                value={form.embedding_model}
                onChange={(e) => setForm((f) => ({ ...f, embedding_model: e.target.value }))}
                placeholder="text-embedding-3-small"
              />
              <p className="muted" style={{ fontSize: 'var(--text-xs)', margin: '0.25rem 0 0' }}>
                不同 embedding 模型输出维度不同，切换模型需重建知识库索引。
              </p>
            </label>
            <div className="form-actions">
              <button type="submit" disabled={createMutation.isPending || updateMutation.isPending}>
                {createMutation.isPending || updateMutation.isPending ? '保存中...' : '保存'}
              </button>
              <button type="button" className="button-secondary" onClick={cancelForm}>
                取消
              </button>
            </div>
            {(createMutation.isError || updateMutation.isError) && (
              <div className="error-box">
                {((createMutation.error ?? updateMutation.error) as Error).message}
              </div>
            )}
          </form>
        </div>
      )}

      {/* ---- Provider List ---- */}
      {providersQuery.isLoading && <p className="system-tip">加载中...</p>}
      {providersQuery.isError && (
        <div className="error-box">{(providersQuery.error as Error).message}</div>
      )}

      {providers.length === 0 && !providersQuery.isLoading && !showForm && (
        <div className="card">
          <p className="muted">尚未配置任何模型提供商。点击上方按钮添加。</p>
        </div>
      )}

      {providers.map((p) => (
        <div key={p.provider_id} className="card">
          <div className="list-item" style={{ alignItems: 'flex-start' }}>
            <div style={{ flex: 1 }}>
              <h3 style={{ marginBottom: '0.25rem' }}>
                {p.name}
                {p.is_default && <span className="pill success" style={{ marginLeft: '0.5rem' }}>默认</span>}
              </h3>
              <p className="muted" style={{ fontSize: 'var(--text-xs)', margin: '0 0 0.25rem' }}>
                {p.api_base_url}
              </p>
              <p className="muted" style={{ fontSize: 'var(--text-xs)', margin: 0 }}>
                模型: {p.default_model}
                {p.embedding_model && <> &middot; Embedding: {p.embedding_model}</>}
                {' '}&middot; Key: {p.api_key_masked || '****'}
              </p>
            </div>
            <div style={{ display: 'flex', gap: '0.5rem', flexShrink: 0 }}>
              <button
                type="button"
                className="button-secondary"
                onClick={() => testMutation.mutate(p.provider_id)}
                disabled={testMutation.isPending}
              >
                {testMutation.isPending && testResult?.id !== p.provider_id ? '...' : '测试'}
              </button>
              {!p.is_default && (
                <button type="button" className="button-secondary" onClick={() => setDefault(p.provider_id)}>
                  设为默认
                </button>
              )}
              <button type="button" className="button-secondary" onClick={() => startEdit(p)}>
                编辑
              </button>
              <button
                type="button"
                className="button-secondary"
                onClick={() => {
                  if (confirm(`确定删除 "${p.name}"?`)) deleteMutation.mutate(p.provider_id);
                }}
              >
                删除
              </button>
            </div>
          </div>

          {/* Test result */}
          {testResult?.id === p.provider_id && (
            <div
              className={testResult.result.success ? 'status-banner healthy' : 'status-banner unhealthy'}
              style={{ marginTop: '0.75rem' }}
            >
              {testResult.result.success ? (
                <>
                  <span className="status-dot up" />
                  连接成功 ({testResult.result.latency_ms}ms) &middot; 回复: "{testResult.result.answer}"
                </>
              ) : (
                <>
                  <span className="status-dot down" />
                  连接失败: {testResult.result.error}
                </>
              )}
            </div>
          )}
        </div>
      ))}
    </section>
  );
}
