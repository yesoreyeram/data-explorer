import { useState, type FormEvent } from "react";

import { Modal } from "../../components/Modal";
import type { Connection, ConnectionType } from "../../api/types";

interface ConnectionFormModalProps {
  connection?: Connection;
  onClose: () => void;
  onSubmit: (input: {
    name: string;
    type: ConnectionType;
    description: string;
    config: Record<string, unknown>;
    secret?: Record<string, string>;
  }) => Promise<void>;
}

const TYPE_LABELS: Record<ConnectionType, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  rest: "REST API",
};

export function ConnectionFormModal({ connection, onClose, onSubmit }: ConnectionFormModalProps) {
  const isEdit = Boolean(connection);
  const [type, setType] = useState<ConnectionType>(connection?.type ?? "postgres");
  const [name, setName] = useState(connection?.name ?? "");
  const [description, setDescription] = useState(connection?.description ?? "");

  const cfg = (connection?.config ?? {}) as Record<string, string>;
  const [host, setHost] = useState(cfg.host ?? "");
  const [port, setPort] = useState(cfg.port ?? (type === "mysql" ? "3306" : "5432"));
  const [database, setDatabase] = useState(cfg.database ?? "");
  const [dbUser, setDbUser] = useState(cfg.user ?? "");
  const [sslMode, setSslMode] = useState(cfg.sslMode ?? "prefer");
  const [password, setPassword] = useState("");

  const [baseUrl, setBaseUrl] = useState(cfg.baseUrl ?? "");
  const [authType, setAuthType] = useState(cfg.authType ?? "none");
  const [apiKeyHeader, setApiKeyHeader] = useState(cfg.apiKeyHeader ?? "X-Api-Key");
  const [apiKey, setApiKey] = useState("");
  const [bearerToken, setBearerToken] = useState("");
  const [basicUser, setBasicUser] = useState("");
  const [basicPassword, setBasicPassword] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      if (type === "postgres" || type === "mysql") {
        const secret: Record<string, string> = {};
        if (password) secret.password = password;
        await onSubmit({
          name,
          type,
          description,
          config: { host, port: Number(port), database, user: dbUser, ...(type === "postgres" ? { sslMode } : {}) },
          secret: Object.keys(secret).length > 0 ? secret : undefined,
        });
      } else {
        const secret: Record<string, string> = {};
        if (authType === "apiKey" && apiKey) secret.apiKey = apiKey;
        if (authType === "bearer" && bearerToken) secret.bearerToken = bearerToken;
        if (authType === "basic" && (basicUser || basicPassword)) {
          secret.username = basicUser;
          secret.password = basicPassword;
        }
        await onSubmit({
          name,
          type,
          description,
          config: { baseUrl, authType, ...(authType === "apiKey" ? { apiKeyHeader } : {}) },
          secret: Object.keys(secret).length > 0 ? secret : undefined,
        });
      }
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save connection");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      title={isEdit ? "Edit connection" : "New connection"}
      onClose={onClose}
      width={520}
      footer={
        <>
          <button className="btn" type="button" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-primary" type="submit" form="connection-form" disabled={submitting}>
            {submitting ? "Saving..." : "Save connection"}
          </button>
        </>
      }
    >
      {error && <div className="error-banner">{error}</div>}
      <form id="connection-form" onSubmit={handleSubmit}>
        <div className="field">
          <label htmlFor="conn-type">Type</label>
          <select
            id="conn-type"
            className="select"
            value={type}
            disabled={isEdit}
            onChange={(e) => setType(e.target.value as ConnectionType)}
          >
            {Object.entries(TYPE_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </div>

        <div className="field">
          <label htmlFor="conn-name">Name</label>
          <input id="conn-name" className="input" required value={name} onChange={(e) => setName(e.target.value)} />
        </div>

        <div className="field">
          <label htmlFor="conn-desc">Description</label>
          <input id="conn-desc" className="input" value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>

        {(type === "postgres" || type === "mysql") && (
          <>
            <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", gap: 12 }}>
              <div className="field">
                <label htmlFor="conn-host">Host</label>
                <input id="conn-host" className="input" required value={host} onChange={(e) => setHost(e.target.value)} />
              </div>
              <div className="field">
                <label htmlFor="conn-port">Port</label>
                <input id="conn-port" className="input" type="number" value={port} onChange={(e) => setPort(e.target.value)} />
              </div>
            </div>
            <div className="field">
              <label htmlFor="conn-db">Database</label>
              <input id="conn-db" className="input" required value={database} onChange={(e) => setDatabase(e.target.value)} />
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <div className="field">
                <label htmlFor="conn-user">User</label>
                <input id="conn-user" className="input" required value={dbUser} onChange={(e) => setDbUser(e.target.value)} />
              </div>
              <div className="field">
                <label htmlFor="conn-password">Password {isEdit && <span className="field-hint">(leave blank to keep)</span>}</label>
                <input
                  id="conn-password"
                  className="input"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
            </div>
            {type === "postgres" && (
              <div className="field">
                <label htmlFor="conn-sslmode">SSL mode</label>
                <select id="conn-sslmode" className="select" value={sslMode} onChange={(e) => setSslMode(e.target.value)}>
                  <option value="disable">disable</option>
                  <option value="prefer">prefer</option>
                  <option value="require">require</option>
                  <option value="verify-full">verify-full</option>
                </select>
              </div>
            )}
            <p className="field-hint">
              Use a read-only database role for this connection - it is the primary safeguard against accidental writes.
            </p>
          </>
        )}

        {type === "rest" && (
          <>
            <div className="field">
              <label htmlFor="conn-baseurl">Base URL</label>
              <input
                id="conn-baseurl"
                className="input"
                type="url"
                required
                placeholder="https://api.example.com"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="conn-authtype">Authentication</label>
              <select id="conn-authtype" className="select" value={authType} onChange={(e) => setAuthType(e.target.value)}>
                <option value="none">None</option>
                <option value="bearer">Bearer token</option>
                <option value="apiKey">API key header</option>
                <option value="basic">Basic auth</option>
              </select>
            </div>
            {authType === "bearer" && (
              <div className="field">
                <label htmlFor="conn-bearer">Bearer token {isEdit && <span className="field-hint">(leave blank to keep)</span>}</label>
                <input id="conn-bearer" className="input" type="password" value={bearerToken} onChange={(e) => setBearerToken(e.target.value)} />
              </div>
            )}
            {authType === "apiKey" && (
              <>
                <div className="field">
                  <label htmlFor="conn-apikeyheader">Header name</label>
                  <input id="conn-apikeyheader" className="input" value={apiKeyHeader} onChange={(e) => setApiKeyHeader(e.target.value)} />
                </div>
                <div className="field">
                  <label htmlFor="conn-apikey">API key {isEdit && <span className="field-hint">(leave blank to keep)</span>}</label>
                  <input id="conn-apikey" className="input" type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} />
                </div>
              </>
            )}
            {authType === "basic" && (
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
                <div className="field">
                  <label htmlFor="conn-basicuser">Username</label>
                  <input id="conn-basicuser" className="input" value={basicUser} onChange={(e) => setBasicUser(e.target.value)} />
                </div>
                <div className="field">
                  <label htmlFor="conn-basicpassword">Password</label>
                  <input
                    id="conn-basicpassword"
                    className="input"
                    type="password"
                    value={basicPassword}
                    onChange={(e) => setBasicPassword(e.target.value)}
                  />
                </div>
              </div>
            )}
          </>
        )}
      </form>
    </Modal>
  );
}
