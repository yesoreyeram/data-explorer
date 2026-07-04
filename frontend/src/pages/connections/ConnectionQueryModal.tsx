import { useState } from "react";

import { Modal } from "../../components/Modal";
import { DataFrameView } from "../../components/DataFrameView";
import { QuerySpecFields } from "../../components/QuerySpecFields";
import { queryConnection } from "../../api/connections";
import { extractErrorMessage } from "../../api/client";
import type { Connection, DataFrame } from "../../api/types";
import { buildQuerySpec, defaultQueryFormState } from "../../lib/querySpec";
import { Button, Field, Input } from "../../components/ui";

export function ConnectionQueryModal({ connection, onClose }: { connection: Connection; onClose: () => void }) {
  const cloudService = String(connection.config.service ?? "");

  const [form, setForm] = useState(defaultQueryFormState());
  const [result, setResult] = useState<DataFrame | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  function patchForm(patch: Partial<typeof form>) {
    setForm((prev) => ({ ...prev, ...patch }));
  }

  async function runQuery() {
    setRunning(true);
    setError(null);
    setResult(null);
    try {
      const res = await queryConnection(connection.id, buildQuerySpec(connection.type, form));
      setResult(res);
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setRunning(false);
    }
  }

  return (
    <Modal title={`Run query – ${connection.name}`} onClose={onClose} width={720}>
      <QuerySpecFields type={connection.type} cloudService={cloudService} value={form} onChange={patchForm} />

      <div className="toolbar" style={{ marginBottom: 12 }}>
        <Field htmlFor="query-limit" label="Row limit" style={{ margin: 0, width: 120 }}>
          <Input
            id="query-limit"
            type="number"
            min={1}
            max={10000}
            value={form.rowLimit}
            onChange={(e) => patchForm({ rowLimit: Number(e.target.value) })}
          />
        </Field>
        <Button variant="primary" onClick={runQuery} disabled={running} style={{ alignSelf: "flex-end" }}>
          {running ? "Running..." : "Run"}
        </Button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {result && <DataFrameView frame={result} />}
    </Modal>
  );
}
