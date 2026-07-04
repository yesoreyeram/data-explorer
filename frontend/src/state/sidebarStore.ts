import { create } from "zustand";

const STORAGE_KEY = "de-sidebar-collapsed";

function loadInitial(): boolean {
  return localStorage.getItem(STORAGE_KEY) === "1";
}

interface SidebarState {
  collapsed: boolean;
  toggle: () => void;
}

export const useSidebarStore = create<SidebarState>((set, get) => ({
  collapsed: loadInitial(),
  toggle: () => {
    const next = !get().collapsed;
    localStorage.setItem(STORAGE_KEY, next ? "1" : "0");
    set({ collapsed: next });
  },
}));
