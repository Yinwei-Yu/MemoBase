import { FormEvent, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiDelete, apiGet, apiPost, apiUpload } from '../lib/api/client';
import type { DocumentContent, DocumentItem, Pagination, Task } from '../lib/types/api';

type ListResp = { items: DocumentItem[]; pagination: Pagination };
type UploadItem = { doc_id: string; task_id: string; status: string; file_name: string; kb_id: string; created_at: string };
type UploadResp = { items: UploadItem[]; uploaded_count: number };

export default function DocumentsPage() {
  const { kbId = '' } = useParams();
  const queryClient = useQueryClient();
  const [files, setFiles] = useState<File[]>([]);
  const [taskId, setTaskId] = useState('');
  const [previewDoc, setPreviewDoc] = useState<{ id: string; title: string } | null>(null);

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

  const docContentQuery = useQuery({
    queryKey: ['doc-content', kbId, previewDoc?.id],
    queryFn: () => apiGet<DocumentContent>(`/knowledge-bases/${kbId}/documents/${previewDoc?.id}/content`),
    enabled: !!kbId && !!previewDoc?.id,
  });

  useEffect(() => {
    if (taskQuery.data?.status === 'succeeded') {
      queryClient.invalidateQueries({ queryKey: ['docs', kbId] });
    }
  }, [kbId, queryClient, taskQuery.data?.status]);

  useEffect(() => {
    if (!previewDoc) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setPreviewDoc(null);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [previewDoc]);

  const uploadMutation = useMutation({
    mutationFn: async () => {
      if (files.length === 0) {
        throw new Error('请至少选择一个文件');
      }
      const form = new FormData();
      files.forEach((file) => form.append('files', file));
      form.append('reindex', 'true');
      return apiUpload<UploadResp>(`/knowledge-bases/${kbId}/documents`, form);
    },
    onSuccess: (data) => {
      if (data.items.length > 0) {
        setTaskId(data.items[data.items.length - 1].task_id);
      }
      setFiles([]);
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
    <>
      <section className="page-grid docs-grid">
        <header className="page-head">
          <div>
            <p className="eyebrow">Library Workspace</p>
            <h1>文档与索引</h1>
            <p>上传文档、观察索引任务进度，并维护当前知识库的文档资产。</p>
          </div>
          <Link to={`/chat/${kbId}`} className="button-like">
            进入问答页面
          </Link>
        </header>
        <div className="card">
          <h2>文档上传与索引</h2>
          <form onSubmit={submit} className="stack">
            <label>
              选择文档（可多选）
              <input
                type="file"
                accept=".txt,.md"
                multiple
                onChange={(e) => setFiles(Array.from(e.target.files ?? []))}
              />
            </label>
            {files.length > 0 && (
              <p className="system-tip">已选择 {files.length} 个文件：{files.map((file) => file.name).join('，')}</p>
            )}
            <p className="muted system-tip">当前仅支持 .txt / .md 文本文件解析。</p>
            <button disabled={uploadMutation.isPending || files.length === 0} type="submit">
              {uploadMutation.isPending ? '上传中...' : '上传并建立索引'}
            </button>
          </form>
          {uploadMutation.isError && <div className="error-box">{(uploadMutation.error as Error).message}</div>}
          {taskId && (
            <div className="task-box">
              <h4>任务状态</h4>
              <p className="system-tip">task_id: {taskId}</p>
              <p className="system-tip">status: {taskQuery.data?.status ?? 'loading'}</p>
              <p className="system-tip">progress: {taskQuery.data?.progress ?? 0}%</p>
              {taskQuery.data?.error_message && <div className="error-box">{taskQuery.data.error_message}</div>}
            </div>
          )}
        </div>

        <div className="card">
          <h2>文档列表</h2>
          {docsQuery.isLoading && <p className="system-tip">加载中...</p>}
          {docsQuery.isError && <div className="error-box">{(docsQuery.error as Error).message}</div>}
          {!docsQuery.isLoading && docsQuery.data?.items.length === 0 && <p className="muted system-tip">暂无文档</p>}
          <div className="list">
            {docsQuery.data?.items.map((doc) => (
              <div key={doc.doc_id} className="list-item">
                <div>
                  <h3>{doc.title}</h3>
                  <p>{doc.file_name}</p>
                  <small>status: {doc.status}</small>
                </div>
                <div className="row-gap">
                  <button onClick={() => setPreviewDoc({ id: doc.doc_id, title: doc.title })}>查看原文</button>
                  <button onClick={() => reindexMutation.mutate(doc.doc_id)}>重建索引</button>
                  <button onClick={() => deleteMutation.mutate(doc.doc_id)}>删除</button>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {previewDoc && (
        <div className="doc-modal-backdrop" onClick={() => setPreviewDoc(null)}>
          <div className="doc-modal" onClick={(event) => event.stopPropagation()}>
            <div className="doc-modal-head">
              <div>
                <h3>{docContentQuery.data?.title ?? previewDoc.title}</h3>
                <p className="system-tip">{docContentQuery.data?.file_name ?? previewDoc.id}</p>
              </div>
              <button type="button" className="doc-modal-close" onClick={() => setPreviewDoc(null)}>
                关闭
              </button>
            </div>
            <div className="doc-modal-content">
              {docContentQuery.isLoading && <p className="system-tip">加载原文中...</p>}
              {docContentQuery.isError && <div className="error-box">{(docContentQuery.error as Error).message}</div>}
              {!docContentQuery.isLoading && !docContentQuery.isError && (
                <pre>{docContentQuery.data?.content_text || '(文档暂无内容)'}</pre>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
