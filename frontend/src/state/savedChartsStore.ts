import { create } from "zustand";

import type { DataFrame } from "../api/types";

const STORAGE_KEY = "de-saved-charts";
const MAX_ROWS = 200;

export type ChartKind = "bar" | "line" | "area" | "pie";
export type ViewMode = "table" | "chart" | "split";

export interface ChartConfig {
  title: string;
  kind: ChartKind;
  xKey: string;
  yKeys: string[];
  viewMode: ViewMode;
}

export interface SavedChart {
  id: string;
  savedAt: string;
  sourceLabel: string;
  config: ChartConfig;
  frame: DataFrame;
}

interface SavedChartsState {
  items: SavedChart[];
  saveChart: (sourceLabel: string, frame: DataFrame, config: ChartConfig) => void;
  removeChart: (id: string) => void;
}

function loadItems(): SavedChart[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const parsed = raw ? (JSON.parse(raw) as unknown) : [];
    return Array.isArray(parsed) ? (parsed as SavedChart[]) : [];
  } catch {
    return [];
  }
}

function persist(items: SavedChart[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
}

export const useSavedChartsStore = create<SavedChartsState>((set, get) => ({
  items: loadItems(),
  saveChart: (sourceLabel, frame, config) => {
    const snapshot: SavedChart = {
      id: crypto.randomUUID(),
      savedAt: new Date().toISOString(),
      sourceLabel,
      config,
      frame: { ...frame, rows: frame.rows.slice(0, MAX_ROWS) },
    };
    const items = [snapshot, ...get().items].slice(0, 8);
    persist(items);
    set({ items });
  },
  removeChart: (id) => {
    const items = get().items.filter((item) => item.id !== id);
    persist(items);
    set({ items });
  },
}));
