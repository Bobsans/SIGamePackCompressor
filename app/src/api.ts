import axios from "axios";
import type { Capabilities, CompressionJob, CompressionOptions, UploadedPack } from "@/types";
import { urljoin } from "@/utils";

export const API_URL = "<![API_SERVER_URL]!>";
const client = axios.create({ baseURL: API_URL, timeout: 20_000 });
export const makeUrl = (path: string) => urljoin(API_URL, path);

export const getCapabilities = async () => (await client.get<Capabilities>("/api/options")).data;
export const getUpload = async (id: string) => (await client.get<UploadedPack>(`/api/uploads/${encodeURIComponent(id)}`)).data;
export const getJob = async (id: string, signal?: AbortSignal) =>
  (await client.get<CompressionJob>(`/api/jobs/${encodeURIComponent(id)}`, signal ? { signal } : {})).data;
export const startJob = async (id: string, options: CompressionOptions) =>
  (await client.post<CompressionJob>(`/api/uploads/${encodeURIComponent(id)}/jobs`, options)).data;
export const cancelJob = async (id: string) =>
  (await client.post<CompressionJob>(`/api/jobs/${encodeURIComponent(id)}/cancel`)).data;

export function openJobStream(id: string) {
  const url = new URL(makeUrl(`/api/jobs/${encodeURIComponent(id)}/ws`), window.location.href);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return new WebSocket(url);
}

export async function uploadPack(file: File, signal: AbortSignal, progress: (percent: number) => void) {
  const form = new FormData();
  form.append("file", file);
  return (await client.post<UploadedPack>("/api/uploads", form, {
    signal,
    timeout: 15 * 60_000,
    onUploadProgress: (event) => {
      if (event.total) progress(Math.min(100, Math.round(event.loaded * 100 / event.total)));
    }
  })).data;
}

export function errorMessage(error: unknown): string {
  if (axios.isAxiosError<{ error?: string; detail?: string }>(error)) {
    const detail = error.response?.data?.error ?? error.response?.data?.detail;
    if (detail) return detail;
    if (error.code === "ECONNABORTED") return "Сервер не ответил вовремя — попробуйте ещё раз";
    if (!error.response) return "Не удалось связаться с сервером — проверьте соединение";
  }
  return error instanceof Error ? error.message : "Не удалось выполнить запрос";
}
