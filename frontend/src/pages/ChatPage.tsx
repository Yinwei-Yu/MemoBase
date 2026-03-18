import { FormEvent, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import { apiGet, apiPost } from '../lib/api/client';
import type { ChatResponse, MessageItem, Pagination, TraceItem } from '../lib/types/api';

type MessagesResp = { items: MessageItem[]; pagination: Pagination };

export default function ChatPage() {
  const { kbId = '' } = useParams();
  const [question, setQuestion] = useState('');
  const [sessionId, setSessionId] = useState('');
  const [traceId, setTraceId] = useState('');

  const chatMutation = useMutation({
    mutationFn: () =>
      apiPost<ChatResponse, Record<string, unknown>>('/chat/completions', {
        kb_id: kbId,
        session_id: sessionId || undefined,
        question,
        use_agent: true,
        include_trace: true,
        top_k: 6,
      }),
    onSuccess: (data) => {
      setSessionId(data.session_id);
      setTraceId(data.trace_id ?? '');
      setQuestion('');
    },
  });

  const messageQuery = useQuery({
    queryKey: ['messages', sessionId],
    queryFn: () => apiGet<MessagesResp>(`/sessions/${sessionId}/messages`, { page: 1, page_size: 200 }),
    enabled: !!sessionId,
    refetchInterval: 2000,
  });

  const traceQuery = useQuery({
    queryKey: ['trace', traceId],
    queryFn: () => apiGet<TraceItem>(`/chat/traces/${traceId}`),
    enabled: !!traceId,
  });

  const citations = useMemo(() => chatMutation.data?.citations ?? [], [chatMutation.data?.citations]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!question.trim()) {
      return;
    }
    chatMutation.mutate();
  }

  return (
    <section className="page-grid chat-grid">
      <div className="card">
        <div className="inline-between">
          <h2>知识库问答</h2>
          <Link to={`/kbs/${kbId}/documents`}>返回文档</Link>
        </div>
        <form onSubmit={onSubmit} className="stack">
          <textarea
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="输入你的问题，例如：进程与线程的核心区别是什么？"
            rows={4}
          />
          <button disabled={chatMutation.isPending} type="submit">
            {chatMutation.isPending ? '生成中...' : '提问'}
          </button>
        </form>

        {chatMutation.isError && <div className="error-box">{(chatMutation.error as Error).message}</div>}

        {chatMutation.data && (
          <div className="answer-box">
            <h3>答案</h3>
            <p>{chatMutation.data.answer}</p>
            <small>
              tokens: {chatMutation.data.token_usage.total_tokens} | session: {chatMutation.data.session_id}
            </small>
          </div>
        )}

        <div className="citations-box">
          <h3>引用来源</h3>
          {citations.length === 0 && <p className="muted">暂无引用</p>}
          {citations.map((citation) => (
            <article key={citation.chunk_id} className="citation-item">
              <h4>{citation.doc_title}</h4>
              <p>{citation.snippet}</p>
              <small>
                score: {citation.score.toFixed(3)} | source: {citation.retrieval_source}
              </small>
            </article>
          ))}
        </div>
      </div>

      <div className="card">
        <h2>会话记录</h2>
        {!sessionId && <p className="muted">首次提问后会显示当前会话消息。</p>}
        {messageQuery.data?.items.map((msg) => (
          <div key={msg.message_id} className={`msg-item ${msg.role}`}>
            <strong>{msg.role === 'user' ? '你' : '助手'}：</strong>
            <span>{msg.content}</span>
          </div>
        ))}

        <h2>执行轨迹</h2>
        {!traceId && <p className="muted">暂无轨迹</p>}
        {traceQuery.data?.steps.map((step, idx) => (
          <div key={`step-${idx}`} className="trace-item">
            <strong>{String(step.tool ?? 'step')}</strong>
            <pre>{JSON.stringify(step, null, 2)}</pre>
          </div>
        ))}
      </div>
    </section>
  );
}
