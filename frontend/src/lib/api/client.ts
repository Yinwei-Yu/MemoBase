import axios from "axios";
import { useAuthStore } from "../../stores/auth";
import type { ApiErrorBody, ApiSuccess, ChatStreamEvent } from "../types/api";

const API_BASE =
  import.meta.env.VITE_API_BASE ?? "http://localhost:8080/api/v1";

export const client = axios.create({
  baseURL: API_BASE,
});

client.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

function toApiError(err: unknown): Error {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as ApiErrorBody | undefined;
    if (data?.error?.message) {
      return new Error(`${data.error.code}: ${data.error.message}`);
    }
    return new Error(err.message);
  }
  return new Error("unknown api error");
}

export async function apiGet<T>(
  url: string,
  params?: Record<string, unknown>,
): Promise<T> {
  try {
    const resp = await client.get<ApiSuccess<T>>(url, { params });
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}

export async function apiPost<T, P = unknown>(
  url: string,
  payload?: P,
): Promise<T> {
  try {
    const resp = await client.post<ApiSuccess<T>>(url, payload);
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}

export async function apiPatch<T, P = unknown>(
  url: string,
  payload: P,
): Promise<T> {
  try {
    const resp = await client.patch<ApiSuccess<T>>(url, payload);
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}

export async function apiDelete<T>(url: string): Promise<T> {
  try {
    const resp = await client.delete<ApiSuccess<T>>(url);
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}

export async function apiUpload<T>(
  url: string,
  formData: FormData,
): Promise<T> {
  try {
    const resp = await client.post<ApiSuccess<T>>(url, formData);
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}

/**
 * SSE streaming POST request using fetch + ReadableStream.
 * Calls onEvent for each parsed ChatStreamEvent.
 * Returns an abort function.
 */
export function apiPostStream(
  url: string,
  payload: Record<string, unknown>,
  onEvent: (event: ChatStreamEvent) => void,
  onError: (error: Error) => void,
  onComplete: () => void,
): () => void {
  const token = useAuthStore.getState().token;
  const controller = new AbortController();

  fetch(`${API_BASE}${url}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(payload),
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        const errMsg =
          (body as ApiErrorBody)?.error?.message ?? response.statusText;
        onError(new Error(errMsg));
        return;
      }

      const reader = response.body?.getReader();
      if (!reader) {
        onError(new Error("Response body is not readable"));
        return;
      }

      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() ?? "";

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const jsonStr = line.slice(6);
            if (jsonStr === "[DONE]") {
              onComplete();
              return;
            }
            try {
              const event = JSON.parse(jsonStr) as ChatStreamEvent;
              onEvent(event);
            } catch {
              // ignore malformed JSON
            }
          }
        }
      }

      // Process remaining buffer
      if (buffer.startsWith("data: ")) {
        try {
          const event = JSON.parse(buffer.slice(6)) as ChatStreamEvent;
          onEvent(event);
        } catch {
          // ignore
        }
      }

      onComplete();
    })
    .catch((err) => {
      if (err.name !== "AbortError") {
        onError(err instanceof Error ? err : new Error(String(err)));
      }
    });

  return () => controller.abort();
}
