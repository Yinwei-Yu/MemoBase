import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiDelete, apiGet } from '../lib/api/client';
import type { MessageItem, Pagination, SessionItem } from '../lib/types/api';

type SessionResp = { items: SessionItem[]; pagination: Pagination };
type MessageResp = { items: MessageItem[]; pagination: Pagination };

export default function SessionsPage() {
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState('');

  const sessionsQuery = useQuery({
    queryKey: ['sessions'],
    queryFn: () => apiGet<SessionResp>('/sessions', { page: 1, page_size: 100 }),
  });

  const messagesQuery = useQuery({
    queryKey: ['session-messages', selected],
    queryFn: () => apiGet<MessageResp>(`/sessions/${selected}/messages`, { page: 1, page_size: 200 }),
    enabled: !!selected,
  });

  const deleteMutation = useMutation({
    mutationFn: (sessionId: string) => apiDelete<{ deleted: boolean }>(`/sessions/${sessionId}`),
    onSuccess: () => {
      setSelected('');
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
    },
  });

  return (
    <section className="page-grid sessions-grid">
      <header className="page-head">
        <div>
          <p className="eyebrow">Conversation Hub</p>
          <h1>会话管理</h1>
          <p>集中查看历史会话、消息详情，并可快速清理无效会话记录。</p>
        </div>
      </header>
      <div className="card">
        <h2>会话列表</h2>
        {sessionsQuery.isLoading && <p className="system-tip">加载中...</p>}
        {sessionsQuery.isError && <div className="error-box">{(sessionsQuery.error as Error).message}</div>}
        {!sessionsQuery.isLoading && sessionsQuery.data?.items.length === 0 && <p className="muted system-tip">暂无会话</p>}

        <div className="list">
          {sessionsQuery.data?.items.map((session) => (
            <div key={session.session_id} className={`list-item ${selected === session.session_id ? 'selected' : ''}`}>
              <div>
                <h3>{session.title}</h3>
                <small>kb: {session.kb_id}</small>
              </div>
              <div className="row-gap">
                <button onClick={() => setSelected(session.session_id)}>查看消息</button>
                <button onClick={() => deleteMutation.mutate(session.session_id)}>删除</button>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="card">
        <h2>消息</h2>
        {!selected && <p className="muted system-tip">选择一个会话查看消息</p>}
        {messagesQuery.isLoading && <p className="system-tip">加载中...</p>}
        {messagesQuery.data?.items.map((message) => (
          <div key={message.message_id} className={`msg-item ${message.role}`}>
            <strong>{message.role === 'user' ? '你' : '助手'}：</strong>
            <span>{message.content}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
