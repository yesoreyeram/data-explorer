import { api } from "./client";
import type { ConnectionType, DataFrame, QuerySpec } from "./types";

export interface AdhocConnection {
  type: ConnectionType;
  config: Record<string, unknown>;
  secret?: Record<string, string>;
}

export interface ExploreQueryInput {
  /** Exactly one of connectionId/connection should be set. */
  connectionId?: string;
  connection?: AdhocConnection;
  spec: QuerySpec;
}

export async function exploreQuery(input: ExploreQueryInput): Promise<DataFrame> {
  const res = await api.post<DataFrame>("/explore/query", input);
  return res.data;
}
