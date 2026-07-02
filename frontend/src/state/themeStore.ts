import { create } from "zustand";

export type ThemeMode = "light" | "dark" | "system";

const STORAGE_KEY = "de-theme";

function applyTheme(mode: ThemeMode) {
  document.documentElement.setAttribute("data-theme", mode);
}

function loadInitialTheme(): ThemeMode {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "light" || stored === "dark" || stored === "system") {
    return stored;
  }
  return "system";
}

interface ThemeState {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
}

const initial = loadInitialTheme();
applyTheme(initial);

export const useThemeStore = create<ThemeState>((set) => ({
  mode: initial,
  setMode: (mode) => {
    localStorage.setItem(STORAGE_KEY, mode);
    applyTheme(mode);
    set({ mode });
  },
}));
