import type { CloudQuerySpec, ConnectionType, PaginationSpec, QuerySpec } from "../api/types";

// Local editor state for authoring a query, independent of which connection
// type it targets - QuerySpecFields renders the subset that applies, and
// buildQuerySpec() below picks the subset back out into the wire QuerySpec.
// Shared by ConnectionQueryModal (query a saved connection) and ExplorePage
// (query a saved OR temporary connection) so both stay in sync for free.
export interface QueryFormState {
  sql: string;
  path: string;
  method: string;
  gqlQuery: string;
  gqlDataPath: string;
  pagination?: PaginationSpec;
  cloudQuery: CloudQuerySpec;
  rowLimit: number;
}

export function defaultQueryFormState(): QueryFormState {
  return {
    sql: "SELECT 1",
    path: "/",
    method: "GET",
    gqlQuery: "query { __typename }",
    gqlDataPath: "data",
    pagination: undefined,
    cloudQuery: {},
    rowLimit: 100,
  };
}

export function isCloudType(type: ConnectionType | ""): boolean {
  return type === "aws" || type === "gcp" || type === "azure";
}

export function buildQuerySpec(type: ConnectionType | "", state: QueryFormState): QuerySpec {
  if (type === "postgres" || type === "mysql") {
    return { sql: state.sql, rowLimit: state.rowLimit };
  }
  if (type === "graphql") {
    return {
      rowLimit: state.rowLimit,
      graphql: { query: state.gqlQuery, dataPath: state.gqlDataPath },
      pagination: state.pagination,
    };
  }
  if (isCloudType(type)) {
    return { rowLimit: state.rowLimit, cloud: state.cloudQuery };
  }
  return { method: state.method, path: state.path, rowLimit: state.rowLimit, pagination: state.pagination };
}

/** A short, human-readable label for a query - used by ExplorePage's recent
 * queries list so an entry is recognizable at a glance without expanding it. */
export function summarizeQuery(type: ConnectionType | "", state: QueryFormState): string {
  const oneLine = (s: string) => s.trim().replace(/\s+/g, " ");
  if (type === "postgres" || type === "mysql") return oneLine(state.sql).slice(0, 80);
  if (type === "graphql") return oneLine(state.gqlQuery).slice(0, 80);
  if (isCloudType(type)) return oneLine(state.cloudQuery.query ?? state.cloudQuery.tableName ?? state.cloudQuery.bucket ?? "").slice(0, 80);
  return `${state.method} ${state.path}`;
}
