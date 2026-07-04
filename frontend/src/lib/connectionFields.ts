import { useState } from "react";

import type { AuthType, CatalogEntry, Connection, ConnectionType } from "../api/types";

function str(v: unknown): string {
  return typeof v === "string" ? v : v == null ? "" : String(v);
}

export interface ConnectionFieldsInit {
  connection?: Connection;
  /** Catalog pick to prefill from - only meaningful when `connection` is unset. */
  prefill?: CatalogEntry;
}

/** All the per-type connection config/secret state (DB/HTTP+auth/cloud),
 * shared by ConnectionFormModal (persists via onSubmit) and ExplorePage's
 * temporary-connection mode (never persisted - sent inline with the query
 * instead). See ConnectionTypeConfigFields for the matching form fields. */
export function useConnectionFields({ connection, prefill }: ConnectionFieldsInit = {}) {
  const [type, setTypeRaw] = useState<ConnectionType>(connection?.type ?? prefill?.type ?? "postgres");

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
  const [httpConfig, setHttpConfig] = useState<Record<string, unknown>>(connection ? initialCfg : (prefill?.authConfig ?? {}));
  const [cloudConfig, setCloudConfig] = useState<Record<string, unknown>>(initialCfg);
  const [secret, setSecret] = useState<Record<string, string>>({});

  function patchConfig(patch: Record<string, unknown>) {
    setHttpConfig((prev) => ({ ...prev, ...patch }));
  }
  function patchCloudConfig(patch: Record<string, unknown>) {
    setCloudConfig((prev) => ({ ...prev, ...patch }));
  }
  function patchSecret(patch: Record<string, string>) {
    setSecret((prev) => ({ ...prev, ...patch }));
  }

  // Config/secret are keyed dicts shared across all connection types (e.g.
  // "service" means something different for aws/gcp/azure) - clear them on
  // type switch so a field from the previous type can't leak into the new
  // type's rendered form or payload.
  function setType(newType: ConnectionType) {
    setTypeRaw(newType);
    setHttpConfig({});
    setCloudConfig({});
    setSecret({});
    setAuthType("none");
    setBaseUrl("");
    setEndpoint("");
  }

  function buildConfigAndSecret(): { config: Record<string, unknown>; secret?: Record<string, string> } {
    if (type === "postgres" || type === "mysql") {
      const dbSecret: Record<string, string> = {};
      if (password) dbSecret.password = password;
      return {
        config: { host, port: Number(port), database, user: dbUser, ...(type === "postgres" ? { sslMode } : {}) },
        secret: Object.keys(dbSecret).length > 0 ? dbSecret : undefined,
      };
    }
    if (type === "aws" || type === "gcp" || type === "azure") {
      return { config: cloudConfig, secret: Object.keys(secret).length > 0 ? secret : undefined };
    }
    return {
      config: { ...httpConfig, authType, ...(type === "rest" ? { baseUrl } : { endpoint }) },
      secret: Object.keys(secret).length > 0 ? secret : undefined,
    };
  }

  return {
    type,
    setType,
    host,
    setHost,
    port,
    setPort,
    database,
    setDatabase,
    dbUser,
    setDbUser,
    sslMode,
    setSslMode,
    password,
    setPassword,
    baseUrl,
    setBaseUrl,
    endpoint,
    setEndpoint,
    authType,
    setAuthType,
    httpConfig,
    patchConfig,
    cloudConfig,
    patchCloudConfig,
    secret,
    patchSecret,
    buildConfigAndSecret,
  };
}

export type ConnectionFields = ReturnType<typeof useConnectionFields>;
