import { create } from "zustand";

import * as authApi from "../api/auth";
import { setAccessToken, setUnauthorizedHandler } from "../api/client";
import type { User } from "../api/types";

interface AuthState {
  user: User | null;
  permissions: string[];
  roles: string[];
  status: "idle" | "loading" | "authenticated" | "anonymous";
  error: string | null;
  bootstrap: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, displayName: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  hasPermission: (permission: string) => boolean;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  permissions: [],
  roles: [],
  status: "idle",
  error: null,

  // On app load there is no access token in memory (it never survives a
  // full page reload by design), so we try the refresh cookie once to
  // silently re-establish a session.
  bootstrap: async () => {
    set({ status: "loading" });
    try {
      const res = await authApi.refresh();
      setAccessToken(res.accessToken);
      const meRes = await authApi.me();
      set({
        user: meRes.user,
        roles: meRes.roles,
        permissions: meRes.permissions,
        status: "authenticated",
      });
    } catch {
      setAccessToken(null);
      set({ status: "anonymous", user: null, roles: [], permissions: [] });
    }
  },

  login: async (email, password) => {
    set({ error: null });
    const res = await authApi.login(email, password);
    setAccessToken(res.accessToken);
    const meRes = await authApi.me();
    set({
      user: meRes.user,
      roles: meRes.roles,
      permissions: meRes.permissions,
      status: "authenticated",
    });
  },

  register: async (email, displayName, password) => {
    set({ error: null });
    const res = await authApi.register(email, displayName, password);
    setAccessToken(res.accessToken);
    set({
      user: res.user,
      roles: res.roles,
      permissions: res.permissions,
      status: "authenticated",
    });
  },

  logout: async () => {
    try {
      await authApi.logout();
    } finally {
      setAccessToken(null);
      set({ user: null, roles: [], permissions: [], status: "anonymous" });
    }
  },

  hasPermission: (permission) => get().permissions.includes(permission),
}));

setUnauthorizedHandler(() => {
  setAccessToken(null);
  useAuthStore.setState({ user: null, roles: [], permissions: [], status: "anonymous" });
});
