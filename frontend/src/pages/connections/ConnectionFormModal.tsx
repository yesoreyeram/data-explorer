import { useState, type FormEvent } from "react";

import { Modal } from "../../components/Modal";
import type { AuthType, CatalogEntry, Connection, ConnectionType } from "../../api/types";
import { AUTH_TYPE_OPTIONS, AuthTypeFields } from "./AuthTypeFields";
import { CloudConnectionFields } from "./CloudConnectionFields";

interface ConnectionFormModalProps {
  connection?: Connection;
  /** Prefills a new (non-edit) form from a catalog pick - see CatalogBrowserModal. Ignored when editing. */
  catalogEntry?: CatalogEntry;
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
  graphql: "GraphQL API",
  aws: "AWS",
  gcp: "Google Cloud",
  azure: "Microsoft Azure",
};

function str(v: unknown): string {
  return typeof v === "string" ? v : v == null ? "" : String(v);
}

export function ConnectionFormModal({ connection, catalogEntry, onClose, onSubmit }: ConnectionFormModalProps) {
  const isEdit = Boolean(connection);
  // catalogEntry only ever applies to a brand-new connection, never an edit.
  const prefill = isEdit ? undefined : catalogEntry;
  const [type, setType] = useState<ConnectionType>(connection?.type ?? prefill?.type ?? "postgres");
  const [name, setName] = useState(connection?.name ?? prefill?.name ?? "");
  const [description, setDescription] = useState(connection?.description ?? prefill?.description ?? "");

  const initialCfg = (connection?.config ?? {}) as Record<string, unknown>;
  const [host, setHost] = useState(str(initialCfg.host));
  const [port, setPort] = useState(str(initialCfg.port) || (type === "mysql" ? "3306" : "5432"));
  const [database, setDatabase] = useState(str(initialCfg.database));
  const [dbUser, setDbUser] = useState(str(initialCfg.user));
  const [sslMode, setSslMode] = useState(str(initialCfg.sslMode) || "prefer");
  const [password, setPassword] = useState("");

  const [baseUrl, setBaseUrl] = useState(connection ? str(initialCfg.baseUrl) : (prefill?.baseUrl ?? ""));
  const [endpoint, setEndpoint] = useState(connection ? str(initialCfg.endpoint) : (prefill?.endpoint ?? ""));
  const [authType, setAuthType] = useState<AuthType>(
    connection ? ((initialCfg.authType as AuthType) ?? "none") : (prefill?.authType ?? "none"),
  );
  const [httpConfig, setHttpConfig] = useState<Record<string, unknown>>(
    connection ? initialCfg : (prefill?.authConfig ?? {}),
  );
  const [cloudConfig, setCloudConfig] = useState<Record<string, unknown>>(initialCfg);
  const [secret, setSecret] = useState<Record<string, string>>({});

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function patchConfig(patch: Record<string, unknown>) {
    setHttpConfig((prev) => ({ ...prev, ...patch }));
  }
  function patchCloudConfig(patch: Record<string, unknown>) {
    setCloudConfig((prev) => ({ ...prev, ...patch }));
  }
  function patchSecret(patch: Record<string, string>) {
    setSecret((prev) => ({ ...prev, ...patch }));
  }
  function handleTypeChange(newType: ConnectionType) {
    setType(newType);
    // Config/secret are keyed dicts shared across all connection types (e.g. "service"
    // means something different for aws/gcp/azure) - clear them on type switch so a
    // field from the previous type can't leak into the new type's rendered form or payload.
    setHttpConfig({});
    setCloudConfig({});
    setSecret({});
    setAuthType("none");
    setBaseUrl("");
    setEndpoint("");
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      if (type === "postgres" || type === "mysql") {
        const dbSecret: Record<string, string> = {};
        if (password) dbSecret.password = password;
        await onSubmit({
          name,
          type,
          description,
          config: { host, port: Number(port), database, user: dbUser, ...(type === "postgres" ? { sslMode } : {}) },
          secret: Object.keys(dbSecret).length > 0 ? dbSecret : undefined,
        });
      } else if (type === "aws" || type === "gcp" || type === "azure") {
        await onSubmit({
          name,
          type,
          description,
          config: cloudConfig,
          secret: Object.keys(secret).length > 0 ? secret : undefined,
        });
      } else {
        const config = { ...httpConfig, authType, ...(type === "rest" ? { baseUrl } : { endpoint }) };
        await onSubmit({
          name,
          type,
          description,
          config,
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

  const isHTTP = type === "rest" || type === "graphql";
  const isCloud = type === "aws" || type === "gcp" || type === "azure";

  return (
    <Modal
      title={isEdit ? "Edit connection" : "New connection"}
      onClose={onClose}
      width={560}
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
            onChange={(e) => handleTypeChange(e.target.value as ConnectionType)}
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

        {isHTTP && (
          <>
            <div className="field">
              <label htmlFor="conn-url">{type === "rest" ? "Base URL" : "GraphQL endpoint"}</label>
              <input
                id="conn-url"
                className="input"
                type="url"
                required
                placeholder={type === "rest" ? "https://api.example.com" : "https://api.example.com/graphql"}
                value={type === "rest" ? baseUrl : endpoint}
                onChange={(e) => (type === "rest" ? setBaseUrl(e.target.value) : setEndpoint(e.target.value))}
              />
            </div>
            <div className="field">
              <label htmlFor="conn-authtype">Authentication</label>
              <select id="conn-authtype" className="select" value={authType} onChange={(e) => setAuthType(e.target.value as AuthType)}>
                {AUTH_TYPE_OPTIONS.map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </div>
            <AuthTypeFields
              authType={authType}
              config={httpConfig}
              onConfigChange={patchConfig}
              secret={secret}
              onSecretChange={patchSecret}
              isEdit={isEdit}
            />
            {prefill && prefill.type === type && prefill.docsUrl && (
              <p className="field-hint">
                Get credentials from{" "}
                <a href={prefill.docsUrl} target="_blank" rel="noreferrer">
                  {prefill.name}'s auth docs
                </a>
                .
              </p>
            )}
          </>
        )}

        {isCloud && (
          <CloudConnectionFields
            type={type}
            config={cloudConfig}
            onConfigChange={patchCloudConfig}
            secret={secret}
            onSecretChange={patchSecret}
            isEdit={isEdit}
          />
        )}
      </form>
    </Modal>
  );
}
