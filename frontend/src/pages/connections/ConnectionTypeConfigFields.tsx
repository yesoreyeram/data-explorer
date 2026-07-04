import type { AuthType, CatalogEntry, ConnectionType } from "../../api/types";
import type { ConnectionFields } from "../../lib/connectionFields";
import { AUTH_TYPE_OPTIONS, AuthTypeFields } from "./AuthTypeFields";
import { CloudConnectionFields } from "./CloudConnectionFields";
import { Field, Input, Select } from "../../components/ui";

interface ConnectionTypeConfigFieldsProps extends ConnectionFields {
  isEdit: boolean;
  /** Catalog pick this form was opened from, for the "get credentials" docs hint. */
  prefill?: CatalogEntry;
}

/** The per-type config/secret fields (Postgres/MySQL, REST/GraphQL + auth,
 * AWS/GCP/Azure) - shared by ConnectionFormModal and ExplorePage's
 * temporary-connection mode. Pairs with useConnectionFields, which owns all
 * the state this renders. */
export function ConnectionTypeConfigFields({ type, isEdit, prefill, ...f }: ConnectionTypeConfigFieldsProps) {
  const isHTTP = type === "rest" || type === "graphql";
  const isCloud = type === "aws" || type === "gcp" || type === "azure";

  return (
    <>
      {(type === "postgres" || type === "mysql") && (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", gap: 12 }}>
            <Field htmlFor="conn-host" label="Host">
              <Input id="conn-host" required value={f.host} onChange={(e) => f.setHost(e.target.value)} />
            </Field>
            <Field htmlFor="conn-port" label="Port">
              <Input id="conn-port" type="number" value={f.port} onChange={(e) => f.setPort(e.target.value)} />
            </Field>
          </div>
          <Field htmlFor="conn-db" label="Database">
            <Input id="conn-db" required value={f.database} onChange={(e) => f.setDatabase(e.target.value)} />
          </Field>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <Field htmlFor="conn-user" label="User">
              <Input id="conn-user" required value={f.dbUser} onChange={(e) => f.setDbUser(e.target.value)} />
            </Field>
            <Field
              htmlFor="conn-password"
              label={
                <>
                  Password {isEdit && <span className="field-hint">(leave blank to keep)</span>}
                </>
              }
            >
              <Input
                id="conn-password"
                type="password"
                autoComplete="new-password"
                value={f.password}
                onChange={(e) => f.setPassword(e.target.value)}
              />
            </Field>
          </div>
          {type === "postgres" && (
            <Field htmlFor="conn-sslmode" label="SSL mode">
              <Select id="conn-sslmode" value={f.sslMode} onChange={(e) => f.setSslMode(e.target.value)}>
                <option value="disable">disable</option>
                <option value="prefer">prefer</option>
                <option value="require">require</option>
                <option value="verify-full">verify-full</option>
              </Select>
            </Field>
          )}
          <p className="field-hint">
            Use a read-only database role for this connection - it is the primary safeguard against accidental writes.
          </p>
        </>
      )}

      {isHTTP && (
        <>
          <Field htmlFor="conn-url" label={type === "rest" ? "Base URL" : "GraphQL endpoint"}>
            <Input
              id="conn-url"
              type="url"
              required
              placeholder={type === "rest" ? "https://api.example.com" : "https://api.example.com/graphql"}
              value={type === "rest" ? f.baseUrl : f.endpoint}
              onChange={(e) => (type === "rest" ? f.setBaseUrl(e.target.value) : f.setEndpoint(e.target.value))}
            />
          </Field>
          <Field htmlFor="conn-authtype" label="Authentication">
            <Select id="conn-authtype" value={f.authType} onChange={(e) => f.setAuthType(e.target.value as AuthType)}>
              {AUTH_TYPE_OPTIONS.map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </Select>
          </Field>
          <AuthTypeFields
            authType={f.authType}
            config={f.httpConfig}
            onConfigChange={f.patchConfig}
            secret={f.secret}
            onSecretChange={f.patchSecret}
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
          type={type as ConnectionType}
          config={f.cloudConfig}
          onConfigChange={f.patchCloudConfig}
          secret={f.secret}
          onSecretChange={f.patchSecret}
          isEdit={isEdit}
        />
      )}
    </>
  );
}
