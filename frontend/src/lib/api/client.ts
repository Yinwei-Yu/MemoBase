import axios from 'axios';
import { useAuthStore } from '../../stores/auth';
import type { ApiErrorBody, ApiSuccess } from '../types/api';

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080/api/v1';

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
  return new Error('unknown api error');
}

export async function apiGet<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  try {
    const resp = await client.get<ApiSuccess<T>>(url, { params });
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}

export async function apiPost<T, P = unknown>(url: string, payload?: P): Promise<T> {
  try {
    const resp = await client.post<ApiSuccess<T>>(url, payload);
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}

export async function apiPatch<T, P = unknown>(url: string, payload: P): Promise<T> {
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

export async function apiUpload<T>(url: string, formData: FormData): Promise<T> {
  try {
    const resp = await client.post<ApiSuccess<T>>(url, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return resp.data.data;
  } catch (err) {
    throw toApiError(err);
  }
}
