import { FormEvent, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiDelete, apiGet, apiPost, apiUpload } from '../lib/api/client';
import type { DocumentItem, Pagination, Task } from '../lib/types/api';

type ListResp = { items: DocumentItem[]; pagination: Pagination };
type UploadResp = { doc_id: string; task_id: string; status: string; file_name: string; kb_id: string; created_at: string };

export default function DocumentsPage() {
  const { kbId = '' } = useParams();
  const queryClient = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [taskId, setTaskId] = useState('');

  const docsQuery = useQuery({
    queryKey: ['docs', kbId],
    queryFn: () => apiGet<ListResp>(`/knowledge-bases/${kbId}/documents`, { page: 1, page_size: 100 }),
    enabled: !!kbId,
  });

  const taskQuery = useQuery({
    queryKey: ['task', taskId],
    queryFn: () => apiGet<Task>(`/tasks/${taskId}`),
    enabled: !!taskId,
    refetchInterval: (q) => (q.state.data?.status === 'succeeded' || q.state.data?.status === 'failed' ? false : 1500),
  });

  useEffect(() => {
    if (taskQuery.data?.status === 'succeeded') {
      queryClient.invalidateQueries({ queryKey: ['docs', kbId] });
    }
  }, [kbId, queryClient, taskQuery.data?.status]);

  const uploadMutation = useMutation({
    mutationFn: async () => {
      if (!file) {
        throw new Error('请选择文件');
      }
      const form = new FormData();
      form.append('file', file);
      form.append('reindex', 'true');
      return apiUpload<UploadResp>(`/knowledge-bases/${kbId}/documents`, form);
    },
    onSuccess: (data) => {
      setTaskId(data.task_id);
      queryClient.invalidateQueries({ queryKey: ['docs', kbId] });
    },
  });

  const reindexMutation = useMutation({
    mutationFn: (docId: string) => apiPost<{ task_id: string }>(`/knowledge-bases/${kbId}/documents/${docId}/reindex`),
    onSuccess: (data) => setTaskId(data.task_id),
  });

  const deleteMutation = useMutation({
    mutationFn: (docId: string) => apiDelete<{ deleted: boolean }>(`/knowledge-bases/${kbId}/documents/${docId}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['docs', kbId] }),
  });

  function submit(event: FormEvent) {
    event.preventDefault();
    uploadMutation.mutate();
  }

  return (
    <section className="page-grid docs-grid">
      <div className="card">
        <h2>文档上传与索引</h2>
        <form onSubmit={submit} className="stack">
          <label>
            选择文档
            <input type="file" accept=".txt,.md" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
          </label>
          <p className="muted">当前仅支持 .txt / .md 文本文件解析。</p>
          <button disabled={uploadMutation.isPending} type="submit">
            {uploadMutation.isPending ? '上传中...' : '上传并建立索引'}
          </button>
        </form>
        {uploadMutation.isError && <div className="error-box">{(uploadMutation.error as Error).message}</div>}
        {taskId && (
          <div className="task-box">
            <h4>任务状态</h4>
            <p>task_id: {taskId}</p>
            <p>status: {taskQuery.data?.status ?? 'loading'}</p>
            <p>progress: {taskQuery.data?.progress ?? 0}%</p>
            {taskQuery.data?.error_message && <div className="error-box">{taskQuery.data.error_message}</div>}
          </div>
        )}
        <p>
          <Link to={`/chat/${kbId}`}>进入问答页面</Link>
        </p>
      </div>

      <div className="card">
        <h2>文档列表</h2>
        {docsQuery.isLoading && <p>加载中...</p>}
        {docsQuery.isError && <div className="error-box">{(docsQuery.error as Error).message}</div>}
        {!docsQuery.isLoading && docsQuery.data?.items.length === 0 && <p className="muted">暂无文档</p>}
        <div className="list">
          {docsQuery.data?.items.map((doc) => (
            <div key={doc.doc_id} className="list-item">
              <div>
                <h3>{doc.title}</h3>
                <p>{doc.file_name}</p>
                <small>status: {doc.status}</small>
              </div>
              <div className="row-gap">
                <button onClick={() => reindexMutation.mutate(doc.doc_id)}>重建索引</button>
                <button onClick={() => deleteMutation.mutate(doc.doc_id)}>删除</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
