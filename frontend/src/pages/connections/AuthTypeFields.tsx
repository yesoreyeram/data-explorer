import type { AuthType } from "../../api/types";

const AUTH_TYPE_LABELS: Record<AuthType, string> = {
  none: "None",
  basic: "Basic",
  bearer: "Bearer token",
  apiKey: "API key",
  digest: "Digest",
  oauth2ClientCredentials: "OAuth2 - Client Credentials",
  oauth2RefreshToken: "OAuth2 - Refresh Token",
  jwt: "JWT (self-signed)",
  workloadIdentity: "Workload Identity Federation",
  kerberos: "Kerberos / SPNEGO",
};

export const AUTH_TYPE_OPTIONS = Object.entries(AUTH_TYPE_LABELS) as [AuthType, string][];

interface AuthTypeFieldsProps {
  authType: AuthType;
  config: Record<string, unknown>;
  onConfigChange: (patch: Record<string, unknown>) => void;
  secret: Record<string, string>;
  onSecretChange: (patch: Record<string, string>) => void;
  isEdit: boolean;
}

const secretHint = (isEdit: boolean) => (isEdit ? " (leave blank to keep current value)" : "");

function str(v: unknown): string {
  return typeof v === "string" ? v : v == null ? "" : String(v);
}

export function AuthTypeFields({ authType, config, onConfigChange, secret, onSecretChange, isEdit }: AuthTypeFieldsProps) {
  switch (authType) {
    case "none":
      return null;

    case "basic":
      return (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div className="field">
            <label htmlFor="auth-user">Username</label>
            <input id="auth-user" className="input" value={secret.username ?? ""} onChange={(e) => onSecretChange({ username: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="auth-pass">Password{secretHint(isEdit)}</label>
            <input id="auth-pass" className="input" type="password" value={secret.password ?? ""} onChange={(e) => onSecretChange({ password: e.target.value })} />
          </div>
        </div>
      );

    case "bearer":
      return (
        <div className="field">
          <label htmlFor="auth-bearer">Bearer token{secretHint(isEdit)}</label>
          <input id="auth-bearer" className="input" type="password" value={secret.bearerToken ?? ""} onChange={(e) => onSecretChange({ bearerToken: e.target.value })} />
        </div>
      );

    case "apiKey":
      return (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div className="field">
              <label htmlFor="auth-apikeyloc">Location</label>
              <select
                id="auth-apikeyloc"
                className="select"
                value={str(config.apiKeyLocation) || "header"}
                onChange={(e) => onConfigChange({ apiKeyLocation: e.target.value })}
              >
                <option value="header">Header</option>
                <option value="query">Query parameter</option>
              </select>
            </div>
            <div className="field">
              <label htmlFor="auth-apikeyname">{str(config.apiKeyLocation) === "query" ? "Query param name" : "Header name"}</label>
              <input
                id="auth-apikeyname"
                className="input"
                placeholder={str(config.apiKeyLocation) === "query" ? "api_key" : "X-Api-Key"}
                value={str(config.apiKeyLocation) === "query" ? str(config.apiKeyParam) : str(config.apiKeyHeader)}
                onChange={(e) =>
                  onConfigChange(str(config.apiKeyLocation) === "query" ? { apiKeyParam: e.target.value } : { apiKeyHeader: e.target.value })
                }
              />
            </div>
          </div>
          <div className="field">
            <label htmlFor="auth-apikey">API key{secretHint(isEdit)}</label>
            <input id="auth-apikey" className="input" type="password" value={secret.apiKey ?? ""} onChange={(e) => onSecretChange({ apiKey: e.target.value })} />
          </div>
        </>
      );

    case "digest":
      return (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div className="field">
            <label htmlFor="auth-digest-user">Username</label>
            <input id="auth-digest-user" className="input" value={secret.username ?? ""} onChange={(e) => onSecretChange({ username: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="auth-digest-pass">Password{secretHint(isEdit)}</label>
            <input id="auth-digest-pass" className="input" type="password" value={secret.password ?? ""} onChange={(e) => onSecretChange({ password: e.target.value })} />
          </div>
        </div>
      );

    case "oauth2ClientCredentials":
    case "oauth2RefreshToken":
      return (
        <>
          <div className="field">
            <label htmlFor="auth-oauth2-tokenurl">Token URL</label>
            <input
              id="auth-oauth2-tokenurl"
              className="input"
              placeholder="https://auth.example.com/oauth/token"
              value={str(config.oauth2TokenUrl)}
              onChange={(e) => onConfigChange({ oauth2TokenUrl: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="auth-oauth2-scopes">Scopes (comma-separated)</label>
            <input
              id="auth-oauth2-scopes"
              className="input"
              value={Array.isArray(config.oauth2Scopes) ? (config.oauth2Scopes as string[]).join(", ") : ""}
              onChange={(e) =>
                onConfigChange({ oauth2Scopes: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) })
              }
            />
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div className="field">
              <label htmlFor="auth-oauth2-clientid">Client ID{secretHint(isEdit)}</label>
              <input id="auth-oauth2-clientid" className="input" value={secret.oauth2ClientId ?? ""} onChange={(e) => onSecretChange({ oauth2ClientId: e.target.value })} />
            </div>
            <div className="field">
              <label htmlFor="auth-oauth2-clientsecret">Client secret{secretHint(isEdit)}</label>
              <input
                id="auth-oauth2-clientsecret"
                className="input"
                type="password"
                value={secret.oauth2ClientSecret ?? ""}
                onChange={(e) => onSecretChange({ oauth2ClientSecret: e.target.value })}
              />
            </div>
          </div>
          {authType === "oauth2RefreshToken" && (
            <div className="field">
              <label htmlFor="auth-oauth2-refresh">Refresh token{secretHint(isEdit)}</label>
              <input
                id="auth-oauth2-refresh"
                className="input"
                type="password"
                value={secret.oauth2RefreshToken ?? ""}
                onChange={(e) => onSecretChange({ oauth2RefreshToken: e.target.value })}
              />
            </div>
          )}
        </>
      );

    case "jwt":
      return (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div className="field">
              <label htmlFor="auth-jwt-alg">Algorithm</label>
              <select id="auth-jwt-alg" className="select" value={str(config.jwtAlgorithm) || "HS256"} onChange={(e) => onConfigChange({ jwtAlgorithm: e.target.value })}>
                <option value="HS256">HS256 (shared secret)</option>
                <option value="RS256">RS256 (RSA private key)</option>
              </select>
            </div>
            <div className="field">
              <label htmlFor="auth-jwt-ttl">Token TTL (seconds)</label>
              <input id="auth-jwt-ttl" className="input" type="number" value={Number(config.jwtTtlSeconds) || 300} onChange={(e) => onConfigChange({ jwtTtlSeconds: Number(e.target.value) })} />
            </div>
          </div>
          <div className="field">
            <label htmlFor="auth-jwt-key">{str(config.jwtAlgorithm) === "RS256" ? "RSA private key (PEM)" : "Signing secret"}{secretHint(isEdit)}</label>
            <textarea
              id="auth-jwt-key"
              className="textarea"
              rows={str(config.jwtAlgorithm) === "RS256" ? 4 : 1}
              value={secret.jwtSigningKey ?? ""}
              onChange={(e) => onSecretChange({ jwtSigningKey: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="auth-jwt-claims">Claims (JSON object)</label>
            <textarea
              id="auth-jwt-claims"
              className="textarea"
              rows={2}
              placeholder='{"sub": "my-service", "iss": "data-explorer"}'
              value={config.jwtClaims ? JSON.stringify(config.jwtClaims) : ""}
              onChange={(e) => {
                try {
                  onConfigChange({ jwtClaims: e.target.value ? JSON.parse(e.target.value) : undefined });
                } catch {
                  // ignore invalid JSON while typing
                }
              }}
            />
          </div>
        </>
      );

    case "workloadIdentity":
      return (
        <>
          <div className="field">
            <label htmlFor="auth-wif-endpoint">Token exchange endpoint (RFC 8693)</label>
            <input
              id="auth-wif-endpoint"
              className="input"
              placeholder="https://sts.example.com/token"
              value={str(config.workloadIdentityTokenEndpoint)}
              onChange={(e) => onConfigChange({ workloadIdentityTokenEndpoint: e.target.value })}
            />
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div className="field">
              <label htmlFor="auth-wif-audience">Audience</label>
              <input id="auth-wif-audience" className="input" value={str(config.workloadIdentityAudience)} onChange={(e) => onConfigChange({ workloadIdentityAudience: e.target.value })} />
            </div>
            <div className="field">
              <label htmlFor="auth-wif-scope">Scope</label>
              <input id="auth-wif-scope" className="input" value={str(config.workloadIdentityScope)} onChange={(e) => onConfigChange({ workloadIdentityScope: e.target.value })} />
            </div>
          </div>
          <div className="field">
            <label htmlFor="auth-wif-path">Subject token file path</label>
            <input
              id="auth-wif-path"
              className="input"
              placeholder="/var/run/secrets/tokens/identity-token"
              value={str(config.workloadIdentitySubjectTokenPath)}
              onChange={(e) => onConfigChange({ workloadIdentitySubjectTokenPath: e.target.value })}
            />
            <span className="field-hint">
              A platform-projected token file (Kubernetes service account, GitHub Actions OIDC, ...) - read fresh on every
              request. Alternatively set a static subject token below.
            </span>
          </div>
          <div className="field">
            <label htmlFor="auth-wif-static">Static subject token (alternative to the file path){secretHint(isEdit)}</label>
            <input
              id="auth-wif-static"
              className="input"
              type="password"
              value={secret.workloadIdentitySubjectToken ?? ""}
              onChange={(e) => onSecretChange({ workloadIdentitySubjectToken: e.target.value })}
            />
          </div>
        </>
      );

    case "kerberos":
      return (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div className="field">
              <label htmlFor="auth-krb-realm">Realm</label>
              <input id="auth-krb-realm" className="input" placeholder="EXAMPLE.COM" value={str(config.kerberosRealm)} onChange={(e) => onConfigChange({ kerberosRealm: e.target.value })} />
            </div>
            <div className="field">
              <label htmlFor="auth-krb-user">Username</label>
              <input id="auth-krb-user" className="input" value={str(config.kerberosUsername)} onChange={(e) => onConfigChange({ kerberosUsername: e.target.value })} />
            </div>
          </div>
          <div className="field">
            <label htmlFor="auth-krb-spn">Service principal name (SPN)</label>
            <input id="auth-krb-spn" className="input" placeholder="HTTP/service.example.com" value={str(config.kerberosSpn)} onChange={(e) => onConfigChange({ kerberosSpn: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="auth-krb-conf">krb5.conf path</label>
            <input id="auth-krb-conf" className="input" placeholder="/etc/krb5.conf" value={str(config.kerberosKrb5ConfPath)} onChange={(e) => onConfigChange({ kerberosKrb5ConfPath: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="auth-krb-keytab">Keytab path (leave blank to use password auth)</label>
            <input id="auth-krb-keytab" className="input" value={str(config.kerberosKeytabPath)} onChange={(e) => onConfigChange({ kerberosKeytabPath: e.target.value })} />
          </div>
          {!str(config.kerberosKeytabPath) && (
            <div className="field">
              <label htmlFor="auth-krb-pass">Password{secretHint(isEdit)}</label>
              <input id="auth-krb-pass" className="input" type="password" value={secret.password ?? ""} onChange={(e) => onSecretChange({ password: e.target.value })} />
            </div>
          )}
        </>
      );

    default:
      return null;
  }
}
