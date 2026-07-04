import { useState, type FormEvent } from "react";

import { Modal } from "../../components/Modal";
import type { CatalogEntry, Connection, ConnectionType } from "../../api/types";
import { useConnectionFields } from "../../lib/connectionFields";
import { ConnectionTypeConfigFields } from "./ConnectionTypeConfigFields";
import { Button, Field, Input } from "../../components/ui";

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

export function ConnectionFormModal({ connection, catalogEntry, onClose, onSubmit }: ConnectionFormModalProps) {
  const isEdit = Boolean(connection);
  // catalogEntry only ever applies to a brand-new connection, never an edit.
  const prefill = isEdit ? undefined : catalogEntry;

  const [name, setName] = useState(connection?.name ?? prefill?.name ?? "");
  const [description, setDescription] = useState(connection?.description ?? prefill?.description ?? "");
  const fields = useConnectionFields({ connection, prefill });

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const { config, secret } = fields.buildConfigAndSecret();
      await onSubmit({ name, type: fields.type, description, config, secret });
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
      width={560}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" type="submit" form="connection-form" disabled={submitting}>
            {submitting ? "Saving..." : "Save connection"}
          </Button>
        </>
      }
    >
      {error && <div className="error-banner">{error}</div>}
      <form id="connection-form" onSubmit={handleSubmit}>
        <Field htmlFor="conn-type" label="Type">
          <select
            id="conn-type"
            className="select"
            value={fields.type}
            disabled={isEdit}
            onChange={(e) => fields.setType(e.target.value as ConnectionType)}
          >
            {Object.entries(TYPE_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </Field>

        <Field htmlFor="conn-name" label="Name">
          <Input id="conn-name" required value={name} onChange={(e) => setName(e.target.value)} />
        </Field>

        <Field htmlFor="conn-desc" label="Description">
          <Input id="conn-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>

        <ConnectionTypeConfigFields {...fields} isEdit={isEdit} prefill={prefill} />
      </form>
    </Modal>
  );
}
