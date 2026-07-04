import axios, { type AxiosRequestConfig } from "axios";

// The backend issues short-lived access tokens (default 15m) that we keep
// in memory only (never localStorage, to reduce XSS token-theft blast
// radius) and a long-lived refresh token in an httpOnly cookie the browser
// manages for us. getAccessToken/setAccessToken form the seam between this
// module and the auth store without creating a hard import-time dependency.
let accessToken: string | null = null;
let onUnauthorized: (() => void) | null = null;

export function getAccessToken(): string | null {
  return accessToken;
}

export function setAccessToken(token: string | null): void {
  accessToken = token;
}

export function setUnauthorizedHandler(handler: () => void): void {
  onUnauthorized = handler;
}

export const API_BASE_URL = (import.meta.env.VITE_API_URL as string | undefined) ?? "/api/v1";

export const api = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
});

api.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers.set("Authorization", `Bearer ${accessToken}`);
  }
  return config;
});

let refreshPromise: Promise<string | null> | null = null;

async function refreshAccessToken(): Promise<string | null> {
  if (!refreshPromise) {
    refreshPromise = axios
      .post<{ accessToken: string }>(`${API_BASE_URL}/auth/refresh`, {}, { withCredentials: true })
      .then((res) => {
        setAccessToken(res.data.accessToken);
        return res.data.accessToken;
      })
      .catch(() => {
        setAccessToken(null);
        return null;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

interface RetryableConfig extends AxiosRequestConfig {
  _retried?: boolean;
}

api.interceptors.response.use(
  (res) => res,
  async (error) => {
    const original = error.config as RetryableConfig | undefined;
    const status = error.response?.status;
    const isAuthEndpoint = original?.url?.includes("/auth/");

    if (status === 401 && original && !original._retried && !isAuthEndpoint) {
      original._retried = true;
      const token = await refreshAccessToken();
      if (token) {
        return api(original);
      }
      onUnauthorized?.();
    }

    return Promise.reject(error);
  },
);

export function extractErrorMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const body = err.response?.data as { error?: { message?: string } } | undefined;
    if (body?.error?.message) return body.error.message;
    if (err.message) return err.message;
  }
  if (err instanceof Error) return err.message;
  return "Something went wrong";
}
