import { api } from "./client";
import type { CatalogEntry } from "./types";

export async function listCatalog(params?: { q?: string; category?: string; type?: string }): Promise<CatalogEntry[]> {
  const search = new URLSearchParams();
  if (params?.q) search.set("q", params.q);
  if (params?.category) search.set("category", params.category);
  if (params?.type) search.set("type", params.type);
  const qs = search.toString();
  const res = await api.get<CatalogEntry[]>(`/catalog/${qs ? `?${qs}` : ""}`);
  return res.data ?? [];
}
