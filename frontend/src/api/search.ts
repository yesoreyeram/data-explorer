import { api } from "./client";
import type { SearchResult } from "./types";

export async function searchWorkspace(q: string) {
  const res = await api.get<SearchResult[]>("/search", { params: { q } });
  return res.data;
}
