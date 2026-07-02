import { useState } from "react";

import { Modal } from "../../components/Modal";
import { DataTable } from "../../components/DataTable";
import { queryConnection } from "../../api/connections";
import { extractErrorMessage } from "../../api/client";
import type { Connection, QueryResult } from "../../api/types";

export function ConnectionQueryModal({ connection, onClose }: { connection: Connection; onClose: () => void }) {
  const isSQL = connection.type === "postgres" || connection.type === "mysql";
  const [sql, setSql] = useState("SELECT 1");
  const [path, setPath] = useState("/");
  const [method, setMethod] = useState("GET");
  const [rowLimit, setRowLimit] = useState(100);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  async function runQuery() {
    setRunning(true);
    setError(null);
    setResult(null);
    try {
      const res = await queryConnection(
        connection.id,
        isSQL ? { sql, rowLimit } : { method, path, rowLimit },
      );
      setResult(res);
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setRunning(false);
    }
  }

  return (
    <Modal title={`Run query – ${connection.name}`} onClose={onClose} width={720}>
      {isSQL ? (
        <div className="field">
          <label htmlFor="query-sql">SQL (SELECT only)</label>
          <textarea id="query-sql" className="textarea" rows={5} value={sql} onChange={(e) => setSql(e.target.value)} />
        </div>
      ) : (
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

      {result && (
        <>
          <p className="field-hint">
            {result.rowCount} row(s){result.truncated ? " (truncated)" : ""}
          </p>
          <DataTable columns={result.columns} rows={result.rows} />
        </>
      )}
    </Modal>
  );
}
