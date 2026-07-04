import { api } from "./client";
import type { User } from "./types";

export interface LoginResponse {
  accessToken: string;
  expiresAt: string;
  user: User;
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  const res = await api.post<LoginResponse>("/auth/login", { email, password });
  return res.data;
}

export async function register(email: string, displayName: string, password: string): Promise<User> {
  const res = await api.post<User>("/auth/register", { email, displayName, password });
  return res.data;
}

export async function refresh(): Promise<LoginResponse> {
  const res = await api.post<LoginResponse>("/auth/refresh");
  return res.data;
}

export async function logout(): Promise<void> {
  await api.post("/auth/logout");
}

export interface MeResponse {
  user: User;
  roles: string[];
  permissions: string[];
}

export async function me(): Promise<MeResponse> {
  const res = await api.get<MeResponse>("/auth/me");
  return res.data;
}
