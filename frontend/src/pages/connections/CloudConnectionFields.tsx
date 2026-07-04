import type { AWSService, AzureService, ConnectionType, GCPService } from "../../api/types";

function str(v: unknown): string {
  return typeof v === "string" ? v : v == null ? "" : String(v);
}

interface CloudConnectionFieldsProps {
  type: ConnectionType; // "aws" | "gcp" | "azure"
  config: Record<string, unknown>;
  onConfigChange: (patch: Record<string, unknown>) => void;
  secret: Record<string, string>;
  onSecretChange: (patch: Record<string, string>) => void;
  isEdit: boolean;
}

const secretHint = (isEdit: boolean) => (isEdit ? " (leave blank to keep current value)" : "");

const AWS_SERVICES: { value: AWSService; label: string }[] = [
  { value: "athena", label: "Athena (SQL)" },
  { value: "cloudwatchLogs", label: "CloudWatch Logs Insights" },
  { value: "dynamodb", label: "DynamoDB" },
  { value: "s3", label: "S3 (object storage)" },
];

const GCP_SERVICES: { value: GCPService; label: string }[] = [
  { value: "bigquery", label: "BigQuery (SQL)" },
  { value: "gcs", label: "Cloud Storage (object storage)" },
];

const AZURE_SERVICES: { value: AzureService; label: string }[] = [
  { value: "logAnalytics", label: "Log Analytics (KQL)" },
  { value: "blobStorage", label: "Blob Storage (object storage)" },
];

export function CloudConnectionFields({ type, config, onConfigChange, secret, onSecretChange, isEdit }: CloudConnectionFieldsProps) {
  if (type === "aws") {
    const service = (str(config.service) || "s3") as AWSService;
    return (
      <>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div className="field">
            <label htmlFor="aws-region">Region</label>
            <input id="aws-region" className="input" placeholder="us-east-1" value={str(config.region)} onChange={(e) => onConfigChange({ region: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="aws-service">Service</label>
            <select id="aws-service" className="select" value={service} onChange={(e) => onConfigChange({ service: e.target.value })}>
              {AWS_SERVICES.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {service === "athena" && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div className="field">
              <label htmlFor="aws-athena-db">Database</label>
              <input id="aws-athena-db" className="input" value={str(config.athenaDatabase)} onChange={(e) => onConfigChange({ athenaDatabase: e.target.value })} />
            </div>
            <div className="field">
              <label htmlFor="aws-athena-wg">Workgroup</label>
              <input id="aws-athena-wg" className="input" placeholder="primary" value={str(config.athenaWorkgroup)} onChange={(e) => onConfigChange({ athenaWorkgroup: e.target.value })} />
            </div>
            <div className="field" style={{ gridColumn: "1 / -1" }}>
              <label htmlFor="aws-athena-output">Query result output location</label>
              <input
                id="aws-athena-output"
                className="input"
                placeholder="s3://my-athena-results/"
                value={str(config.athenaOutputLocation)}
                onChange={(e) => onConfigChange({ athenaOutputLocation: e.target.value })}
              />
            </div>
          </div>
        )}

        <p className="field-hint" style={{ marginBottom: 10 }}>
          Leave credentials blank to use the server's ambient AWS identity (an IAM role attached to the instance/task/pod) instead
          of storing a long-lived key.
        </p>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div className="field">
            <label htmlFor="aws-access-key">Access key ID{secretHint(isEdit)}</label>
            <input id="aws-access-key" className="input" value={secret.accessKeyId ?? ""} onChange={(e) => onSecretChange({ accessKeyId: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="aws-secret-key">Secret access key{secretHint(isEdit)}</label>
            <input id="aws-secret-key" className="input" type="password" value={secret.secretAccessKey ?? ""} onChange={(e) => onSecretChange({ secretAccessKey: e.target.value })} />
          </div>
        </div>
        <div className="field">
          <label htmlFor="aws-session-token">Session token (optional, for temporary credentials){secretHint(isEdit)}</label>
          <input id="aws-session-token" className="input" type="password" value={secret.sessionToken ?? ""} onChange={(e) => onSecretChange({ sessionToken: e.target.value })} />
        </div>

        <p className="field-hint" style={{ marginTop: 10, marginBottom: 10 }}>
          Optionally assume an IAM role on top of the credentials above (or the ambient identity) via AWS STS - useful for
          reaching resources in another account without a separate key per account.
        </p>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div className="field">
            <label htmlFor="aws-role-arn">Role ARN (optional)</label>
            <input
              id="aws-role-arn"
              className="input"
              placeholder="arn:aws:iam::123456789012:role/data-explorer"
              value={str(config.roleArn)}
              onChange={(e) => onConfigChange({ roleArn: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="aws-role-external-id">External ID (optional)</label>
            <input id="aws-role-external-id" className="input" value={str(config.roleExternalId)} onChange={(e) => onConfigChange({ roleExternalId: e.target.value })} />
          </div>
          <div className="field" style={{ gridColumn: "1 / -1" }}>
            <label htmlFor="aws-role-session-name">Role session name (optional)</label>
            <input
              id="aws-role-session-name"
              className="input"
              placeholder="data-explorer"
              value={str(config.roleSessionName)}
              onChange={(e) => onConfigChange({ roleSessionName: e.target.value })}
            />
          </div>
        </div>
      </>
    );
  }

  if (type === "gcp") {
    const service = (str(config.service) || "bigquery") as GCPService;
    return (
      <>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div className="field">
            <label htmlFor="gcp-project">Project ID</label>
            <input id="gcp-project" className="input" value={str(config.projectId)} onChange={(e) => onConfigChange({ projectId: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="gcp-service">Service</label>
            <select id="gcp-service" className="select" value={service} onChange={(e) => onConfigChange({ service: e.target.value })}>
              {GCP_SERVICES.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
            </select>
          </div>
        </div>
        <p className="field-hint" style={{ marginBottom: 10 }}>
          Leave the service account key blank to use Application Default Credentials (a GCE/GKE Workload Identity-bound service
          account) instead of storing a long-lived key.
        </p>
        <div className="field">
          <label htmlFor="gcp-sa-key">Service account key (JSON){secretHint(isEdit)}</label>
          <textarea
            id="gcp-sa-key"
            className="textarea"
            rows={4}
            placeholder='{"type": "service_account", ...}'
            value={secret.serviceAccountKeyJson ?? ""}
            onChange={(e) => onSecretChange({ serviceAccountKeyJson: e.target.value })}
          />
        </div>
        <div className="field">
          <label htmlFor="gcp-impersonate">Impersonate service account (optional)</label>
          <input
            id="gcp-impersonate"
            className="input"
            placeholder="narrower-sa@project.iam.gserviceaccount.com"
            value={str(config.impersonateServiceAccount)}
            onChange={(e) => onConfigChange({ impersonateServiceAccount: e.target.value })}
          />
          <p className="field-hint">
            Scopes the base identity above (or ADC) down to this service account's permissions - it needs
            roles/iam.serviceAccountTokenCreator on this account.
          </p>
        </div>
      </>
    );
  }

  // azure
  const service = (str(config.service) || "logAnalytics") as AzureService;
  return (
    <>
      <div className="field">
        <label htmlFor="azure-service">Service</label>
        <select id="azure-service" className="select" value={service} onChange={(e) => onConfigChange({ service: e.target.value })}>
          {AZURE_SERVICES.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
      </div>

      {service === "logAnalytics" ? (
        <div className="field">
          <label htmlFor="azure-workspace">Workspace ID</label>
          <input id="azure-workspace" className="input" value={str(config.workspaceId)} onChange={(e) => onConfigChange({ workspaceId: e.target.value })} />
        </div>
      ) : (
        <div className="field">
          <label htmlFor="azure-account">Storage account name</label>
          <input id="azure-account" className="input" value={str(config.storageAccount)} onChange={(e) => onConfigChange({ storageAccount: e.target.value })} />
        </div>
      )}

      <p className="field-hint" style={{ marginBottom: 10 }}>
        Leave tenant/client blank to use DefaultAzureCredential (managed identity, AKS workload identity, or an `az login`
        session) instead of a stored service-principal secret.
      </p>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        <div className="field">
          <label htmlFor="azure-tenant">Tenant ID</label>
          <input id="azure-tenant" className="input" value={str(config.tenantId)} onChange={(e) => onConfigChange({ tenantId: e.target.value })} />
        </div>
        <div className="field">
          <label htmlFor="azure-client">Client ID</label>
          <input id="azure-client" className="input" value={str(config.clientId)} onChange={(e) => onConfigChange({ clientId: e.target.value })} />
        </div>
      </div>
      <div className="field">
        <label htmlFor="azure-secret">Client secret{secretHint(isEdit)}</label>
        <input id="azure-secret" className="input" type="password" value={secret.clientSecret ?? ""} onChange={(e) => onSecretChange({ clientSecret: e.target.value })} />
      </div>
      <p className="field-hint" style={{ marginBottom: 10 }}>
        Or authenticate with a client certificate instead of a secret (leave the client secret above blank):
      </p>
      <div className="field">
        <label htmlFor="azure-cert">Client certificate (PEM or PKCS12){secretHint(isEdit)}</label>
        <textarea
          id="azure-cert"
          className="textarea"
          rows={4}
          placeholder="-----BEGIN CERTIFICATE-----..."
          value={secret.clientCertificate ?? ""}
          onChange={(e) => onSecretChange({ clientCertificate: e.target.value })}
        />
      </div>
      <div className="field">
        <label htmlFor="azure-cert-password">Certificate password (optional, for encrypted/PKCS12 certs){secretHint(isEdit)}</label>
        <input
          id="azure-cert-password"
          className="input"
          type="password"
          value={secret.clientCertificatePassword ?? ""}
          onChange={(e) => onSecretChange({ clientCertificatePassword: e.target.value })}
        />
      </div>
    </>
  );
}
