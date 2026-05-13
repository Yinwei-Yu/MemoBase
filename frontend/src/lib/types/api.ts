export type ApiSuccess<T> = {
  data: T;
  request_id: string;
  timestamp: string;
};

export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
    details?: unknown;
  };
  request_id: string;
  timestamp: string;
};

export type Pagination = {
  page: number;
  page_size: number;
  total: number;
};

export type User = {
  user_id: string;
  username: string;
  display_name: string;
};

export type KnowledgeBase = {
  kb_id: string;
  user_id: string;
  name: string;
  description: string;
  tags: string[];
  doc_count: number;
  created_at: string;
  updated_at: string;
};

export type DocumentItem = {
  doc_id: string;
  kb_id: string;
  title: string;
  file_name: string;
  status: 'pending' | 'processing' | 'indexed' | 'failed' | 'deleted';
  created_at: string;
  updated_at: string;
};

export type DocumentContent = {
  doc_id: string;
  kb_id: string;
  title: string;
  file_name: string;
  status: 'pending' | 'processing' | 'indexed' | 'failed' | 'deleted';
  content_text: string;
  created_at: string;
  updated_at: string;
};

export type Task = {
  task_id: string;
  type: string;
  status: 'pending' | 'processing' | 'succeeded' | 'failed';
  progress: number;
  error_code?: string | null;
  error_message?: string | null;
  created_at: string;
  updated_at: string;
};

export type Citation = {
  doc_id: string;
  doc_title: string;
  chunk_id: string;
  snippet: string;
  score: number;
  retrieval_source: string;
};

export type MemoryItem = {
  memory_id: string;
  session_id: string;
  type: string;
  summary: string;
  created_at: string;
};

export type ChatResponse = {
  session_id: string;
  answer: string;
  citations: Citation[];
  memory_used: MemoryItem[];
  trace_id?: string;
  degraded: boolean;
  latency_ms: number;
  token_usage: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
  };
};

export type SessionItem = {
  session_id: string;
  kb_id: string;
  title: string;
  created_at: string;
  updated_at: string;
};

export type MessageItem = {
  message_id: string;
  session_id: string;
  role: 'user' | 'assistant';
  content: string;
  created_at: string;
};

export type TraceItem = {
  trace_id: string;
  session_id: string;
  steps: Array<Record<string, unknown>>;
  created_at: string;
};
