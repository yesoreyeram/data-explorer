import { create } from "zustand";

import * as authApi from "../api/auth";
import { setAccessToken, setUnauthorizedHandler } from "../api/client";
import type { User } from "../api/types";

interface AuthState {
  user: User | null;
  permissions: string[];
  roles: string[];
  folderGrants: Record<string, string[]>;
  status: "idle" | "loading" | "authenticated" | "anonymous";
  error: string | null;
  bootstrap: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, displayName: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  hasPermission: (permission: string) => boolean;
  // hasScopedPermission mirrors backend rbac.Principal.HasScoped: true if
  // the permission is held account-wide, or via a folder-scoped grant on
  // any folder in folderChain (typically a target folder's own id followed
  // by its ancestor ids, so a grant on a parent folder covers descendants).
  hasScopedPermission: (permission: string, folderChain: string[]) => boolean;
  // hasAnyScopedPermission mirrors rbac.Principal.GrantedFolderIDs' "does
  // this permission exist ANYWHERE for this principal" check, with no
  // specific folder in mind - used to decide whether an action's entry
  // point (e.g. "New connection") should be offered at all to a user whose
  // write access is scoped rather than account-wide; which specific
  // folder(s) it applies to is resolved by the create/edit form itself
  // (see FolderSelect), not here.
  hasAnyScopedPermission: (permission: string) => boolean;
}

const emptyAuthState: Pick<AuthState, "user" | "roles" | "permissions" | "folderGrants"> = {
  user: null,
  roles: [],
  permissions: [],
  folderGrants: {},
};

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  permissions: [],
  roles: [],
  folderGrants: {},
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
        folderGrants: meRes.folderGrants ?? {},
        status: "authenticated",
      });
    } catch {
      setAccessToken(null);
      set({ status: "anonymous", ...emptyAuthState });
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
      folderGrants: meRes.folderGrants ?? {},
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
      set({ status: "anonymous", ...emptyAuthState });
    }
  },

  hasPermission: (permission) => get().permissions.includes(permission),

  hasScopedPermission: (permission, folderChain) => {
    if (get().permissions.includes(permission)) return true;
    const { folderGrants } = get();
    return folderChain.some((folderId) => folderGrants[folderId]?.includes(permission));
  },

  hasAnyScopedPermission: (permission) => {
    if (get().permissions.includes(permission)) return true;
    return Object.values(get().folderGrants).some((codes) => codes.includes(permission));
  },
}));

setUnauthorizedHandler(() => {
  setAccessToken(null);
  useAuthStore.setState({ status: "anonymous", ...emptyAuthState });
});
