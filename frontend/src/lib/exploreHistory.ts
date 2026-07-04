import type { QueryFormState } from "./querySpec";

// Recent ad-hoc queries against saved connections, kept client-side only
// (localStorage) - there's no server-side "saved query" concept yet, this is
// just a convenience so a query you already ran doesn't need retyping.
// Deliberately scoped to the "saved connection" mode: a temporary
// connection's non-secret config could technically be replayed too, but
// restoring it means re-populating a dozen separate form fields for
// marginal benefit, so it's left out rather than half-done.
const STORAGE_KEY = "de-explore-history";
const MAX_ENTRIES = 15;

export interface ExploreHistoryEntry {
  id: string;
  ranAt: string;
  connectionId: string;
  connectionLabel: string;
  summary: string;
  queryForm: QueryFormState;
}

export function loadExploreHistory(): ExploreHistoryEntry[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? (parsed as ExploreHistoryEntry[]) : [];
  } catch {
    return [];
  }
}

export function pushExploreHistory(entry: Omit<ExploreHistoryEntry, "id" | "ranAt">): ExploreHistoryEntry[] {
  const next: ExploreHistoryEntry = { ...entry, id: crypto.randomUUID(), ranAt: new Date().toISOString() };
  const history = [next, ...loadExploreHistory()].slice(0, MAX_ENTRIES);
  localStorage.setItem(STORAGE_KEY, JSON.stringify(history));
  return history;
}

export function clearExploreHistory(): ExploreHistoryEntry[] {
  localStorage.removeItem(STORAGE_KEY);
  return [];
}
