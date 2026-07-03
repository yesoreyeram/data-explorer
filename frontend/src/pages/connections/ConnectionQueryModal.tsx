import { useState } from "react";

import { Modal } from "../../components/Modal";
import { DataFrameView } from "../../components/DataFrameView";
import { PaginationFields } from "../../components/PaginationFields";
import { CloudQueryFields } from "../../components/CloudQueryFields";
import { queryConnection } from "../../api/connections";
import { extractErrorMessage } from "../../api/client";
import type { CloudQuerySpec, Connection, DataFrame, PaginationSpec } from "../../api/types";

export function ConnectionQueryModal({ connection, onClose }: { connection: Connection; onClose: () => void }) {
  const isSQL = connection.type === "postgres" || connection.type === "mysql";
  const isGraphQL = connection.type === "graphql";
  const isCloud = connection.type === "aws" || connection.type === "gcp" || connection.type === "azure";
  const cloudService = String(connection.config.service ?? "");

  const [sql, setSql] = useState("SELECT 1");
  const [path, setPath] = useState("/");
  const [method, setMethod] = useState("GET");
  const [gqlQuery, setGqlQuery] = useState("query { __typename }");
  const [gqlDataPath, setGqlDataPath] = useState("data");
  const [pagination, setPagination] = useState<PaginationSpec | undefined>(undefined);
  const [cloudQuery, setCloudQuery] = useState<CloudQuerySpec>({});
  const [rowLimit, setRowLimit] = useState(100);
  const [result, setResult] = useState<DataFrame | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  async function runQuery() {
    setRunning(true);
    setError(null);
    setResult(null);
    try {
      const spec = isSQL
        ? { sql, rowLimit }
        : isGraphQL
          ? { rowLimit, graphql: { query: gqlQuery, dataPath: gqlDataPath }, pagination }
          : isCloud
            ? { rowLimit, cloud: cloudQuery }
            : { method, path, rowLimit, pagination };
      const res = await queryConnection(connection.id, spec);
      setResult(res);
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setRunning(false);
    }
  }

  return (
    <Modal title={`Run query – ${connection.name}`} onClose={onClose} width={720}>
      {isSQL && (
        <div className="field">
          <label htmlFor="query-sql">SQL (SELECT only)</label>
          <textarea id="query-sql" className="textarea" rows={5} value={sql} onChange={(e) => setSql(e.target.value)} />
        </div>
      )}

      {isGraphQL && (
        <>
          <div className="field">
            <label htmlFor="query-gql">GraphQL query</label>
            <textarea id="query-gql" className="textarea" rows={6} value={gqlQuery} onChange={(e) => setGqlQuery(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="query-gql-datapath">Data path</label>
            <input
              id="query-gql-datapath"
              className="input"
              placeholder="data.search"
              value={gqlDataPath}
              onChange={(e) => setGqlDataPath(e.target.value)}
            />
            <span className="field-hint">Where in the response the row(s) live. Relay-style edges/node are unwrapped automatically.</span>
          </div>
          <PaginationFields graphqlOnly value={pagination} onChange={setPagination} />
        </>
      )}

      {isCloud && <CloudQueryFields service={cloudService} value={cloudQuery} onChange={setCloudQuery} />}

      {!isSQL && !isGraphQL && !isCloud && (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "100px 1fr", gap: 12 }}>
            <div className="field">
              <label htmlFor="query-method">Method</label>
              <select id="query-method" className="select" value={method} onChange={(e) => setMethod(e.target.value)}>
                <option>GET</option>
                <option>POST</option>
              </select>
            </div>
            <div className="field">
              <label htmlFor="query-path">Path</label>
              <input id="query-path" className="input" value={path} onChange={(e) => setPath(e.target.value)} />
            </div>
          </div>
          <PaginationFields value={pagination} onChange={setPagination} />
        </>
      )}

      <div className="toolbar" style={{ marginBottom: 12 }}>
        <div className="field" style={{ margin: 0, width: 120 }}>
          <label htmlFor="query-limit">Row limit</label>
          <input
            id="query-limit"
            className="input"
            type="number"
            min={1}
            max={10000}
            value={rowLimit}
            onChange={(e) => setRowLimit(Number(e.target.value))}
          />
        </div>
        <button className="btn btn-primary" type="button" onClick={runQuery} disabled={running} style={{ alignSelf: "flex-end" }}>
          {running ? "Running..." : "Run"}
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {result && <DataFrameView frame={result} />}
    </Modal>
  );
}
