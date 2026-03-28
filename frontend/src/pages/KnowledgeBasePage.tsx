import { FormEvent, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiDelete, apiGet, apiPost } from '../lib/api/client';
import type { KnowledgeBase, Pagination } from '../lib/types/api';

type ListResp = { items: KnowledgeBase[]; pagination: Pagination };

export default function KnowledgeBasePage() {
  const queryClient = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [tags, setTags] = useState('');

  const queryKey = useMemo(() => ['kbs', keyword], [keyword]);
  const { data, isLoading, error } = useQuery({
    queryKey,
    queryFn: () => apiGet<ListResp>('/knowledge-bases', { page: 1, page_size: 50, keyword }),
  });

  const createMutation = useMutation({
    mutationFn: () =>
      apiPost<KnowledgeBase, { name: string; description: string; tags: string[] }>('/knowledge-bases', {
        name,
        description,
        tags: tags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      setName('');
      setDescription('');
      setTags('');
      queryClient.invalidateQueries({ queryKey: ['kbs'] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (kbId: string) => apiDelete<{ deleted: boolean }>(`/knowledge-bases/${kbId}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['kbs'] }),
  });

  function submit(event: FormEvent) {
    event.preventDefault();
    if (!name.trim()) {
      return;
    }
    createMutation.mutate();
  }

  return (
    <section className="page-grid kbs-grid">
      <header className="page-head">
        <div>
          <p className="eyebrow">Knowledge Library</p>
          <h1>知识库中心</h1>
          <p>创建、维护并进入你的知识库，统一管理文档资产与问答场景。</p>
        </div>
      </header>
      <div className="card">
        <h2>创建知识库</h2>
        <form onSubmit={submit} className="stack">
          <label>
            名称
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="操作系统复习" required />
          </label>
          <label>
            描述
            <textarea value={description} onChange={(e) => setDescription(e.target.value)} />
          </label>
          <label>
            标签（逗号分隔）
            <input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="OS,exam" />
          </label>
          <button disabled={createMutation.isPending} type="submit">
            {createMutation.isPending ? '创建中...' : '创建知识库'}
          </button>
          {createMutation.isError && <div className="error-box">{(createMutation.error as Error).message}</div>}
        </form>
      </div>

      <div className="card">
        <div className="inline-between">
          <h2>知识库列表</h2>
          <input
            aria-label="按名称搜索知识库"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="按名称搜索"
          />
        </div>
        {isLoading && <p className="system-tip">加载中...</p>}
        {error && <div className="error-box">{(error as Error).message}</div>}
        {!isLoading && data?.items.length === 0 && <p className="muted system-tip">暂无知识库</p>}
        <div className="list">
          {data?.items.map((kb) => (
            <div key={kb.kb_id} className="list-item">
              <div>
                <h3>{kb.name}</h3>
                <p>{kb.description || '无描述'}</p>
                <small>{kb.doc_count} 个文档</small>
              </div>
              <div className="row-gap">
                <Link to={`/kbs/${kb.kb_id}/documents`} className="button-like">
                  文档
                </Link>
                <Link to={`/chat/${kb.kb_id}`} className="button-like">
                  问答
                </Link>
                <button onClick={() => deleteMutation.mutate(kb.kb_id)}>删除</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
