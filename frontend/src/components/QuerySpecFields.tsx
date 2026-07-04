import type { ConnectionType } from "../api/types";
import type { QueryFormState } from "../lib/querySpec";
import { isCloudType } from "../lib/querySpec";
import { PaginationFields } from "./PaginationFields";
import { CloudQueryFields } from "./CloudQueryFields";
import { Field, Input, Select, Textarea } from "./ui";

interface QuerySpecFieldsProps {
  type: ConnectionType | "";
  cloudService?: string;
  value: QueryFormState;
  onChange: (patch: Partial<QueryFormState>) => void;
  /** Distinguishes field ids when more than one instance renders on a page. */
  idPrefix?: string;
}

/** The query-shape-specific fields (SQL / REST / GraphQL / cloud) - the
 * counterpart to buildQuerySpec() in lib/querySpec.ts. Row limit and the
 * run action are left to the caller since their layout differs between a
 * modal (ConnectionQueryModal) and a full page (ExplorePage). */
export function QuerySpecFields({ type, cloudService = "", value, onChange, idPrefix = "query" }: QuerySpecFieldsProps) {
  const isSQL = type === "postgres" || type === "mysql";
  const isGraphQL = type === "graphql";
  const isCloud = isCloudType(type);

  if (isSQL) {
    return (
      <Field htmlFor={`${idPrefix}-sql`} label="SQL (SELECT only)">
        <Textarea id={`${idPrefix}-sql`} rows={5} value={value.sql} onChange={(e) => onChange({ sql: e.target.value })} />
      </Field>
    );
  }

  if (isGraphQL) {
    return (
      <>
        <Field htmlFor={`${idPrefix}-gql`} label="GraphQL query">
          <Textarea id={`${idPrefix}-gql`} rows={6} value={value.gqlQuery} onChange={(e) => onChange({ gqlQuery: e.target.value })} />
        </Field>
        <Field
          htmlFor={`${idPrefix}-gql-datapath`}
          label="Data path"
          hint="Where in the response the row(s) live. Relay-style edges/node are unwrapped automatically."
        >
          <Input
            id={`${idPrefix}-gql-datapath`}
            placeholder="data.search"
            value={value.gqlDataPath}
            onChange={(e) => onChange({ gqlDataPath: e.target.value })}
          />
        </Field>
        <PaginationFields graphqlOnly value={value.pagination} onChange={(p) => onChange({ pagination: p })} />
      </>
    );
  }

  if (isCloud) {
    return <CloudQueryFields service={cloudService} value={value.cloudQuery} onChange={(c) => onChange({ cloudQuery: c })} />;
  }

  if (!type) {
    return null;
  }

  return (
    <>
      <div style={{ display: "grid", gridTemplateColumns: "100px 1fr", gap: 12 }}>
        <Field htmlFor={`${idPrefix}-method`} label="Method">
          <Select id={`${idPrefix}-method`} value={value.method} onChange={(e) => onChange({ method: e.target.value })}>
            <option>GET</option>
            <option>POST</option>
          </Select>
        </Field>
        <Field htmlFor={`${idPrefix}-path`} label="Path">
          <Input id={`${idPrefix}-path`} value={value.path} onChange={(e) => onChange({ path: e.target.value })} />
        </Field>
      </div>
      <PaginationFields value={value.pagination} onChange={(p) => onChange({ pagination: p })} />
    </>
  );
}
