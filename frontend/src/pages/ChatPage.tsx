import {
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiGet, apiPost, apiPostStream } from "../lib/api/client";
import type {
  AgentStep,
  ChatResponse,
  Citation,
  DocumentContent,
  MessageItem,
  ModelProvider,
  Pagination,
} from "../lib/types/api";

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
  const navigate = useNavigate();
  const { kbId = "", sessionId: urlSessionId = "" } = useParams();
  const [question, setQuestion] = useState("");
  const [sessionId, setSessionId] = useState(urlSessionId);
  const [citationHistory, setCitationHistory] = useState<Citation[][]>([]);
  const [memoryCounts, setMemoryCounts] = useState<number[]>([]);
  const [expandedCitationRows, setExpandedCitationRows] = useState<
    Record<number, boolean>
  >({});
  const [activeCitation, setActiveCitation] =
    useState<CitationModalState>(null);
  const chatStreamRef = useRef<HTMLDivElement | null>(null);
  const [streamingAnswer, setStreamingAnswer] = useState("");
  const [streamingSteps, setStreamingSteps] = useState<AgentStep[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamError, setStreamError] = useState<string | null>(null);
  const abortRef = useRef<(() => void) | null>(null);
  const [selectedProviderId, setSelectedProviderId] = useState<string>("");

  // Fetch available providers
  const providersQuery = useQuery({
    queryKey: ["model-providers"],
    queryFn: () => apiGet<ModelProvider[]>("/model-providers"),
  });
  const providers = providersQuery.data ?? [];

  // Auto-select default provider when providers load
  useEffect(() => {
    if (providers.length > 0 && selectedProviderId === "") {
      const defaultProvider = providers.find((p) => p.is_default);
      if (defaultProvider) {
        setSelectedProviderId(defaultProvider.provider_id);
      }
    }
  }, [providers, selectedProviderId]);

  // Sync sessionId to URL
  useEffect(() => {
    if (sessionId && sessionId !== urlSessionId) {
      navigate(`/chat/${kbId}/${sessionId}`, { replace: true });
    }
  }, [sessionId, urlSessionId, kbId, navigate]);

  const chatMutation = useMutation({
    mutationFn: (currentQuestion: string) =>
      apiPost<ChatResponse, Record<string, unknown>>("/chat/completions", {
        kb_id: kbId,
        session_id: sessionId || undefined,
        question: currentQuestion,
        provider_id: selectedProviderId || undefined,
        use_agent: true,
        include_trace: false,
        top_k: 6,
      }),
    onSuccess: (data) => {
      setSessionId(data.session_id);
      setCitationHistory((prev) => [...prev, data.citations ?? []]);
      setMemoryCounts((prev) => [...prev, data.memory_count ?? 0]);
      setQuestion("");
      queryClient.invalidateQueries({
        queryKey: ["messages", data.session_id],
      });
    },
  });

  // Optimistic user message for immediate display
  const [optimisticUserMsg, setOptimisticUserMsg] = useState<string | null>(null);

  const startStreaming = useCallback(
    (currentQuestion: string) => {
      setIsStreaming(true);
      setStreamingAnswer("");
      setStreamingSteps([]);
      setStreamError(null);
      setOptimisticUserMsg(currentQuestion);
      setQuestion("");

      const abort = apiPostStream(
        "/chat/completions/stream",
        {
          kb_id: kbId,
          session_id: sessionId || undefined,
          question: currentQuestion,
          model: undefined,
          provider_id: selectedProviderId || undefined,
          top_k: 6,
        },
        (event) => {
          switch (event.type) {
            case "step":
              setStreamingSteps((prev) => [
                ...prev,
                {
                  node: event.node,
                  status: event.status as AgentStep["status"],
                  detail: event.detail ?? "",
                },
              ]);
              break;
            case "token":
              setStreamingAnswer((prev) => prev + event.token);
              break;
            case "result":
              setStreamingAnswer(event.answer);
              if (event.citations) {
                setCitationHistory((prev) => [...prev, event.citations ?? []]);
              }
              setMemoryCounts((prev) => [...prev, event.memory_count ?? 0]);
              break;
            case "error":
              setStreamError(event.message);
              break;
          }
        },
        (error) => {
          setStreamError(error.message);
          setIsStreaming(false);
          setOptimisticUserMsg(null);
        },
        () => {
          setIsStreaming(false);
          setOptimisticUserMsg(null);
          if (sessionId) {
            queryClient.invalidateQueries({
              queryKey: ["messages", sessionId],
            });
          } else {
            setTimeout(() => {
              queryClient.invalidateQueries({ queryKey: ["messages"] });
            }, 500);
          }
        },
      );
      abortRef.current = abort;
    },
    [kbId, sessionId, queryClient, selectedProviderId],
  );

  const messageQuery = useQuery({
    queryKey: ["messages", sessionId],
    queryFn: () =>
      apiGet<MessagesResp>(`/sessions/${sessionId}/messages`, {
        page: 1,
        page_size: 200,
      }),
    enabled: !!sessionId,
    refetchInterval: 1200,
  });

  const citationContentQuery = useQuery({
    queryKey: ["chat-citation-content", kbId, activeCitation?.citation.doc_id],
    queryFn: () =>
      apiGet<DocumentContent>(
        `/knowledge-bases/${kbId}/documents/${activeCitation?.citation.doc_id}/content`,
      ),
    enabled: !!kbId && !!activeCitation?.citation.doc_id,
    retry: false,
    staleTime: 5 * 60 * 1000,
  });

  useEffect(() => {
    chatStreamRef.current?.scrollTo({
      top: chatStreamRef.current.scrollHeight,
      behavior: "smooth",
    });
  }, [
    messageQuery.data?.items.length,
    chatMutation.isPending,
    streamingAnswer,
  ]);

  useEffect(() => {
    if (!activeCitation) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setActiveCitation(null);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [activeCitation]);

  const messages = useMemo(
    () => messageQuery.data?.items ?? [],
    [messageQuery.data?.items],
  );

  // During streaming, server messages may overlap with optimistic content.
  // Filter out the last user/assistant messages to avoid duplication and
  // prevent the "appears at top then jumps to bottom" visual glitch.
  const displayMessages = useMemo(() => {
    if (!isStreaming) return messages;

    const filtered = [...messages];

    if (optimisticUserMsg) {
      for (let i = filtered.length - 1; i >= 0; i--) {
        if (filtered[i].role === "user") {
          filtered.splice(i, 1);
          break;
        }
      }
    }

    if (streamingAnswer) {
      for (let i = filtered.length - 1; i >= 0; i--) {
        if (filtered[i].role === "assistant") {
          filtered.splice(i, 1);
          break;
        }
      }
    }

    return filtered;
  }, [messages, isStreaming, optimisticUserMsg, streamingAnswer]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    const currentQuestion = question.trim();
    if (!currentQuestion || chatMutation.isPending || isStreaming) {
      return;
    }
    startStreaming(currentQuestion);
  }

  function onCancelStream() {
    abortRef.current?.();
    setIsStreaming(false);
  }

  function toggleCitationRow(assistantIndex: number) {
    setExpandedCitationRows((prev) => ({
      ...prev,
      [assistantIndex]: !prev[assistantIndex],
    }));
  }

  const isThinking = chatMutation.isPending || isStreaming;

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
            {messages.length === 0 && !isThinking && (
              <p className="muted qa-empty">
                输入问题后，这里会按一问一答展示聊天记录。
              </p>
            )}

            {isThinking && messages.length === 0 && (
              <div className="qa-thinking">
                <div className="typing-indicator">
                  <span className="dot" />
                  <span className="dot" />
                  <span className="dot" />
                </div>
                <span className="qa-thinking-label">
                  {streamingSteps.length > 0
                    ? (streamingSteps[streamingSteps.length - 1]?.detail ??
                      "处理中...")
                    : "AI 正在思考..."}
                </span>
                {isStreaming && (
                  <button
                    type="button"
                    className="qa-cancel-btn"
                    onClick={onCancelStream}
                  >
                    取消
                  </button>
                )}
              </div>
            )}

            {streamError && <div className="error-box">{streamError}</div>}

            {(() => {
              let assistantIndex = -1;
              return displayMessages.map((msg) => {
                if (msg.role === "assistant") {
                  assistantIndex += 1;
                }
                const messageCitations =
                  msg.role === "assistant"
                    ? dedupeCitationsByDoc(
                        citationHistory[assistantIndex] ?? [],
                      )
                    : [];
                const hasCitations = messageCitations.length > 0;
                const memCount =
                  msg.role === "assistant" ? (memoryCounts[assistantIndex] ?? 0) : 0;
                const rowExpanded = !!expandedCitationRows[assistantIndex];
                const visibleCitations = messageCitations.slice(
                  0,
                  MAX_VISIBLE_CITATIONS,
                );

                return (
                  <article
                    key={msg.message_id}
                    className={`qa-item ${msg.role}`}
                  >
                    <p className="qa-item-role">
                      {msg.role === "user" ? "你" : "AI"}
                    </p>
                    <p className="qa-item-content">{msg.content}</p>

                    {msg.role === "assistant" && memCount > 0 && (
                      <p className="muted" style={{ fontSize: 'var(--text-xs)', margin: '0.25rem 0 0' }}>
                        参考了 {memCount} 条记忆
                      </p>
                    )}

                    {msg.role === "assistant" && hasCitations && (
                      <div className="qa-citation-block">
                        <button
                          type="button"
                          className="qa-inline-link"
                          onClick={() => toggleCitationRow(assistantIndex)}
                        >
                          {rowExpanded
                            ? "隐藏引用文档"
                            : `查看引用文档 (${messageCitations.length})`}
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
                                {citation.snippet && (
                                  <span className="qa-doc-link-tooltip">
                                    {citation.snippet.length > 120
                                      ? citation.snippet.slice(0, 120) + "..."
                                      : citation.snippet}
                                  </span>
                                )}
                              </button>
                            ))}
                            {messageCitations.length >
                              MAX_VISIBLE_CITATIONS && (
                              <p className="qa-citation-note">
                                其余{" "}
                                {messageCitations.length -
                                  MAX_VISIBLE_CITATIONS}{" "}
                                条已省略
                              </p>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </article>
                );
              });
            })()}

            {optimisticUserMsg && (
              <article className="qa-item user">
                <p className="qa-item-role">你</p>
                <p className="qa-item-content">{optimisticUserMsg}</p>
              </article>
            )}

            {isStreaming && streamingAnswer && (
              <article className="qa-item assistant streaming">
                <p className="qa-item-role">AI</p>
                <p className="qa-item-content">{streamingAnswer}</p>
              </article>
            )}

            {isThinking && messages.length > 0 && (
              <div className="qa-thinking">
                <div className="typing-indicator">
                  <span className="dot" />
                  <span className="dot" />
                  <span className="dot" />
                </div>
                <span className="qa-thinking-label">
                  {streamingSteps.length > 0
                    ? (streamingSteps[streamingSteps.length - 1]?.detail ??
                      "处理中...")
                    : "AI 正在思考..."}
                </span>
              </div>
            )}
          </div>

          <form onSubmit={onSubmit} className="qa-composer">
            {providers.length > 0 && (
              <div className="qa-provider-select">
                <label>
                  <span>模型:</span>
                  <select
                    value={selectedProviderId}
                    onChange={(e) => setSelectedProviderId(e.target.value)}
                  >
                    <option value="">Ollama (本地)</option>
                    {providers.map((p) => (
                      <option key={p.provider_id} value={p.provider_id}>
                        {p.name} — {p.default_model}
                        {p.is_default ? " (默认)" : ""}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
            )}
            <textarea
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder="输入你的问题..."
              rows={3}
            />
            <button disabled={isThinking || !question.trim()} type="submit">
              {isThinking ? "生成中..." : "发送"}
            </button>
          </form>

          {(chatMutation.isError || messageQuery.isError) && (
            <div className="error-box">
              {((chatMutation.error ?? messageQuery.error) as Error).message}
            </div>
          )}
        </div>
      </section>

      {activeCitation && (
        <div
          className="qa-modal-backdrop"
          onClick={() => setActiveCitation(null)}
        >
          <div
            className="qa-modal"
            role="dialog"
            aria-modal="true"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="qa-modal-head">
              <h3>{activeCitation.citation.doc_title}</h3>
              <button
                type="button"
                className="qa-modal-close"
                onClick={() => setActiveCitation(null)}
              >
                关闭
              </button>
            </div>

            {citationContentQuery.isLoading && (
              <p className="muted">加载原文中...</p>
            )}
            {!citationContentQuery.isLoading && (
              <>
                <pre className="qa-modal-content">
                  {citationContentQuery.data?.content_text ||
                    activeCitation.citation.snippet ||
                    "(暂无可展示内容)"}
                </pre>
                {citationContentQuery.isError && (
                  <p className="muted">
                    完整原文不可用，已展示命中的引用片段。
                  </p>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </>
  );
}
