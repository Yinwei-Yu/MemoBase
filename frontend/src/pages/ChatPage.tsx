import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiGet, apiPost } from '../lib/api/client';
import type { ChatResponse, Citation, DocumentContent, MessageItem, Pagination } from '../lib/types/api';

type MessagesResp = { items: MessageItem[]; pagination: Pagination };
type CitationModalState = { citation: Citation } | null;

const MAX_VISIBLE_CITATIONS = 4;

function dedupeCitationsByDoc(citations: Citation[]): Citation[] {
  const seen = new Set<string>();
  return citations.filter((item) => {
    const key = item.doc_id || item.chunk_id;
    if (!key || seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

export default function ChatPage() {
  const queryClient = useQueryClient();
  const { kbId = '' } = useParams();
  const [question, setQuestion] = useState('');
  const [sessionId, setSessionId] = useState('');
  const [citationHistory, setCitationHistory] = useState<Citation[][]>([]);
  const [expandedCitationRows, setExpandedCitationRows] = useState<Record<number, boolean>>({});
  const [activeCitation, setActiveCitation] = useState<CitationModalState>(null);
  const chatStreamRef = useRef<HTMLDivElement | null>(null);

  const chatMutation = useMutation({
    mutationFn: (currentQuestion: string) =>
      apiPost<ChatResponse, Record<string, unknown>>('/chat/completions', {
        kb_id: kbId,
        session_id: sessionId || undefined,
        question: currentQuestion,
        use_agent: true,
        include_trace: false,
        top_k: 6,
      }),
    onSuccess: (data) => {
      setSessionId(data.session_id);
      setCitationHistory((prev) => [...prev, data.citations ?? []]);
      setQuestion('');
      queryClient.invalidateQueries({ queryKey: ['messages', data.session_id] });
    },
  });

  const messageQuery = useQuery({
    queryKey: ['messages', sessionId],
    queryFn: () => apiGet<MessagesResp>(`/sessions/${sessionId}/messages`, { page: 1, page_size: 200 }),
    enabled: !!sessionId,
    refetchInterval: 1200,
  });

  const citationContentQuery = useQuery({
    queryKey: ['chat-citation-content', kbId, activeCitation?.citation.doc_id],
    queryFn: () => apiGet<DocumentContent>(`/knowledge-bases/${kbId}/documents/${activeCitation?.citation.doc_id}/content`),
    enabled: !!kbId && !!activeCitation?.citation.doc_id,
    retry: false,
    staleTime: 5 * 60 * 1000,
  });

  useEffect(() => {
    chatStreamRef.current?.scrollTo({ top: chatStreamRef.current.scrollHeight, behavior: 'smooth' });
  }, [messageQuery.data?.items.length, chatMutation.isPending]);

  useEffect(() => {
    if (!activeCitation) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setActiveCitation(null);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [activeCitation]);

  const messages = useMemo(() => messageQuery.data?.items ?? [], [messageQuery.data?.items]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    const currentQuestion = question.trim();
    if (!currentQuestion || chatMutation.isPending) {
      return;
    }
    chatMutation.mutate(currentQuestion);
  }

  function toggleCitationRow(assistantIndex: number) {
    setExpandedCitationRows((prev) => ({ ...prev, [assistantIndex]: !prev[assistantIndex] }));
  }

  return (
    <>
      <section className="page-grid chat-grid">
        <div className="card qa-chat-card">
          <div className="qa-chat-head">
            <h1>智能问答</h1>
            <Link to={`/kbs/${kbId}/documents`} className="button-like">
              文档
            </Link>
          </div>

          <div className="qa-stream" ref={chatStreamRef}>
            {messages.length === 0 && !chatMutation.isPending && (
              <p className="muted qa-empty">输入问题后，这里会按一问一答展示聊天记录。</p>
            )}
            {messages.length === 0 && chatMutation.isPending && (
              <p className="muted qa-empty">正在生成回答...</p>
            )}

            {(() => {
              let assistantIndex = -1;
              return messages.map((msg) => {
                if (msg.role === 'assistant') {
                  assistantIndex += 1;
                }
                const messageCitations =
                  msg.role === 'assistant' ? dedupeCitationsByDoc(citationHistory[assistantIndex] ?? []) : [];
                const hasCitations = messageCitations.length > 0;
                const rowExpanded = !!expandedCitationRows[assistantIndex];
                const visibleCitations = messageCitations.slice(0, MAX_VISIBLE_CITATIONS);

                return (
                  <article key={msg.message_id} className={`qa-item ${msg.role}`}>
                    <p className="qa-item-role">{msg.role === 'user' ? '你' : 'AI'}</p>
                    <p className="qa-item-content">{msg.content}</p>

                    {msg.role === 'assistant' && hasCitations && (
                      <div className="qa-citation-block">
                        <button
                          type="button"
                          className="qa-inline-link"
                          onClick={() => toggleCitationRow(assistantIndex)}
                        >
                          {rowExpanded ? '隐藏引用文档' : `查看引用文档 (${messageCitations.length})`}
                        </button>

                        {rowExpanded && (
                          <div className="qa-citation-links">
                            {visibleCitations.map((citation, citationIndex) => (
                              <button
                                type="button"
                                key={citation.chunk_id}
                                className="qa-doc-link"
                                onClick={() => setActiveCitation({ citation })}
                              >
                                [{citationIndex + 1}] {citation.doc_title}
                              </button>
                            ))}
                            {messageCitations.length > MAX_VISIBLE_CITATIONS && (
                              <p className="qa-citation-note">其余 {messageCitations.length - MAX_VISIBLE_CITATIONS} 条已省略</p>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </article>
                );
              });
            })()}
          </div>

          <form onSubmit={onSubmit} className="qa-composer">
            <textarea
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder="输入你的问题..."
              rows={3}
            />
            <button disabled={chatMutation.isPending || !question.trim()} type="submit">
              {chatMutation.isPending ? '生成中...' : '发送'}
            </button>
          </form>

          {(chatMutation.isError || messageQuery.isError) && (
            <div className="error-box">{((chatMutation.error ?? messageQuery.error) as Error).message}</div>
          )}
        </div>
      </section>

      {activeCitation && (
        <div className="qa-modal-backdrop" onClick={() => setActiveCitation(null)}>
          <div className="qa-modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
            <div className="qa-modal-head">
              <h3>{activeCitation.citation.doc_title}</h3>
              <button type="button" className="qa-modal-close" onClick={() => setActiveCitation(null)}>
                关闭
              </button>
            </div>

            {citationContentQuery.isLoading && <p className="muted">加载原文中...</p>}
            {!citationContentQuery.isLoading && (
              <>
                <pre className="qa-modal-content">
                  {citationContentQuery.data?.content_text || activeCitation.citation.snippet || '(暂无可展示内容)'}
                </pre>
                {citationContentQuery.isError && <p className="muted">完整原文不可用，已展示命中的引用片段。</p>}
              </>
            )}
          </div>
        </div>
      )}
    </>
  );
}
