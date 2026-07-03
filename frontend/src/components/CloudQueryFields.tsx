import type { CloudQuerySpec } from "../api/types";

interface CloudQueryFieldsProps {
  /** The cloud service the connection is configured for, e.g. "athena", "s3", "logAnalytics". */
  service: string;
  value: CloudQuerySpec;
  onChange: (patch: CloudQuerySpec) => void;
}

/** Query editor for the aws/gcp/azure connection types, shown once a
 * connection's service is known (each service reads a different subset of
 * CloudQuerySpec - see backend/internal/connections/connector.go). */
export function CloudQueryFields({ service, value, onChange }: CloudQueryFieldsProps) {
  function patch(fields: Partial<CloudQuerySpec>) {
    onChange({ ...value, ...fields });
  }

  if (service === "athena" || service === "bigquery" || service === "logAnalytics") {
    const label = service === "bigquery" ? "SQL query" : service === "athena" ? "SQL query" : "KQL query";
    return (
      <div className="field">
        <label htmlFor="cloud-query">{label}</label>
        <textarea id="cloud-query" className="textarea" rows={6} value={value.query ?? ""} onChange={(e) => patch({ query: e.target.value })} />
      </div>
    );
  }

  if (service === "cloudwatchLogs") {
    return (
      <>
        <div className="field">
          <label htmlFor="cloud-cwl-query">Logs Insights query</label>
          <textarea
            id="cloud-cwl-query"
            className="textarea"
            rows={5}
            placeholder="fields @timestamp, @message | sort @timestamp desc | limit 100"
            value={value.query ?? ""}
            onChange={(e) => patch({ query: e.target.value })}
          />
        </div>
        <div className="field">
          <label htmlFor="cloud-cwl-loggroups">Log group names (comma-separated)</label>
          <input
            id="cloud-cwl-loggroups"
            className="input"
            value={(value.logGroupNames ?? []).join(", ")}
            onChange={(e) =>
              patch({
                logGroupNames: e.target.value
                  .split(",")
                  .map((s) => s.trim())
                  .filter(Boolean),
              })
            }
          />
        </div>
      </>
    );
  }

  if (service === "dynamodb") {
    return (
      <>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
          <div className="field">
            <label htmlFor="cloud-ddb-table">Table name</label>
            <input id="cloud-ddb-table" className="input" value={value.tableName ?? ""} onChange={(e) => patch({ tableName: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="cloud-ddb-index">Index name (optional)</label>
            <input id="cloud-ddb-index" className="input" value={value.indexName ?? ""} onChange={(e) => patch({ indexName: e.target.value })} />
          </div>
        </div>
        <label className="checkbox-row" style={{ marginBottom: 10 }}>
          <input type="checkbox" checked={value.scan ?? false} onChange={(e) => patch({ scan: e.target.checked })} />
          <span>Full table scan (instead of a key-condition query)</span>
        </label>
        {!value.scan && (
          <div className="field">
            <label htmlFor="cloud-ddb-keycond">Key condition expression</label>
            <input
              id="cloud-ddb-keycond"
              className="input"
              placeholder="pk = :pk AND sk > :sk"
              value={value.keyConditionExpression ?? ""}
              onChange={(e) => patch({ keyConditionExpression: e.target.value })}
            />
          </div>
        )}
        <div className="field">
          <label htmlFor="cloud-ddb-filter">Filter expression (optional)</label>
          <input id="cloud-ddb-filter" className="input" value={value.filterExpression ?? ""} onChange={(e) => patch({ filterExpression: e.target.value })} />
        </div>
        <div className="field">
          <label htmlFor="cloud-ddb-values">Expression attribute values (JSON)</label>
          <textarea
            id="cloud-ddb-values"
            className="textarea"
            rows={2}
            placeholder='{":pk": "user#123", ":sk": "2024-01-01"}'
            value={value.expressionAttributeValues ? JSON.stringify(value.expressionAttributeValues) : ""}
            onChange={(e) => {
              try {
                patch({ expressionAttributeValues: e.target.value ? JSON.parse(e.target.value) : undefined });
              } catch {
                // ignore invalid JSON while typing
              }
            }}
          />
        </div>
      </>
    );
  }

  // s3 | gcs | blobStorage
  return (
    <>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
        <div className="field">
          <label htmlFor="cloud-obj-bucket">{service === "blobStorage" ? "Container" : "Bucket"}</label>
          <input id="cloud-obj-bucket" className="input" value={value.bucket ?? ""} onChange={(e) => patch({ bucket: e.target.value })} />
        </div>
        <div className="field">
          <label htmlFor="cloud-obj-format">Format (blank = infer from key)</label>
          <select id="cloud-obj-format" className="select" value={value.format ?? ""} onChange={(e) => patch({ format: (e.target.value || undefined) as CloudQuerySpec["format"] })}>
            <option value="">Auto</option>
            <option value="csv">CSV</option>
            <option value="json">JSON</option>
            <option value="ndjson">NDJSON</option>
          </select>
        </div>
      </div>
      <div className="field">
        <label htmlFor="cloud-obj-key">Object key (reads one file; leave blank to list objects instead)</label>
        <input id="cloud-obj-key" className="input" value={value.key ?? ""} onChange={(e) => patch({ key: e.target.value })} />
      </div>
      {!value.key && (
        <div className="field">
          <label htmlFor="cloud-obj-prefix">Prefix (for listing)</label>
          <input id="cloud-obj-prefix" className="input" value={value.prefix ?? ""} onChange={(e) => patch({ prefix: e.target.value })} />
        </div>
      )}
      {value.format === "csv" && (
        <div className="field">
          <label htmlFor="cloud-obj-delim">CSV delimiter</label>
          <input id="cloud-obj-delim" className="input" placeholder="," value={value.delimiter ?? ""} onChange={(e) => patch({ delimiter: e.target.value })} />
        </div>
      )}
    </>
  );
}
