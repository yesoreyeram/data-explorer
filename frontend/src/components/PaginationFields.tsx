import type { PaginationSpec, PaginationStrategy } from "../api/types";

interface PaginationFieldsProps {
  value: PaginationSpec | undefined;
  onChange: (value: PaginationSpec | undefined) => void;
  /** GraphQL connections only support "graphqlRelay"; REST supports every other strategy. */
  graphqlOnly?: boolean;
}

const REST_STRATEGIES: { value: PaginationStrategy; label: string }[] = [
  { value: "none", label: "None (single request)" },
  { value: "offset", label: "Offset / limit" },
  { value: "page", label: "Page number" },
  { value: "cursor", label: "Cursor" },
  { value: "linkHeader", label: "Link header (RFC 5988)" },
];

const GRAPHQL_STRATEGIES: { value: PaginationStrategy; label: string }[] = [
  { value: "none", label: "None (single request)" },
  { value: "graphqlRelay", label: "Relay cursor connection" },
];

export function PaginationFields({ value, onChange, graphqlOnly }: PaginationFieldsProps) {
  const strategy = value?.strategy ?? "none";
  const options = graphqlOnly ? GRAPHQL_STRATEGIES : REST_STRATEGIES;

  function patch(fields: Partial<PaginationSpec>) {
    onChange({ strategy, ...value, ...fields });
  }

  function setStrategy(next: PaginationStrategy) {
    if (next === "none") {
      onChange(undefined);
    } else {
      onChange({ ...value, strategy: next });
    }
  }

  return (
    <>
      <div className="field">
        <label htmlFor="pg-strategy">Pagination</label>
        <select id="pg-strategy" className="select" value={strategy} onChange={(e) => setStrategy(e.target.value as PaginationStrategy)}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>

      {(strategy === "offset" || strategy === "page" || strategy === "cursor" || strategy === "linkHeader") && (
        <div className="field">
          <label htmlFor="pg-itemspath">Items path</label>
          <input
            id="pg-itemspath"
            className="input"
            placeholder="data.items (blank = response body is the array)"
            value={value?.itemsPath ?? ""}
            onChange={(e) => patch({ itemsPath: e.target.value })}
          />
        </div>
      )}

      {strategy === "offset" && (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8 }}>
          <div className="field">
            <label htmlFor="pg-offsetparam">Offset param</label>
            <input id="pg-offsetparam" className="input" placeholder="offset" value={value?.offsetParam ?? ""} onChange={(e) => patch({ offsetParam: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pg-limitparam">Limit param</label>
            <input id="pg-limitparam" className="input" placeholder="limit" value={value?.limitParam ?? ""} onChange={(e) => patch({ limitParam: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pg-pagesize">Page size</label>
            <input id="pg-pagesize" className="input" type="number" value={value?.pageSize ?? 50} onChange={(e) => patch({ pageSize: Number(e.target.value) })} />
          </div>
        </div>
      )}

      {strategy === "page" && (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8 }}>
          <div className="field">
            <label htmlFor="pg-pageparam">Page param</label>
            <input id="pg-pageparam" className="input" placeholder="page" value={value?.pageParam ?? ""} onChange={(e) => patch({ pageParam: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pg-pagesizeparam">Page size param</label>
            <input id="pg-pagesizeparam" className="input" placeholder="per_page" value={value?.pageSizeParam ?? ""} onChange={(e) => patch({ pageSizeParam: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pg-pagesize2">Page size</label>
            <input id="pg-pagesize2" className="input" type="number" value={value?.pageSize ?? 50} onChange={(e) => patch({ pageSize: Number(e.target.value) })} />
          </div>
        </div>
      )}

      {strategy === "cursor" && (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
          <div className="field">
            <label htmlFor="pg-cursorparam">Cursor query param</label>
            <input id="pg-cursorparam" className="input" placeholder="cursor" value={value?.cursorParam ?? ""} onChange={(e) => patch({ cursorParam: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pg-cursorpath">Next-cursor response path</label>
            <input id="pg-cursorpath" className="input" placeholder="meta.next_cursor" value={value?.cursorPath ?? ""} onChange={(e) => patch({ cursorPath: e.target.value })} />
          </div>
        </div>
      )}

      {strategy === "graphqlRelay" && (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8 }}>
          <div className="field">
            <label htmlFor="pg-gqlcursor">Cursor variable</label>
            <input id="pg-gqlcursor" className="input" placeholder="after" value={value?.graphqlCursorVariable ?? ""} onChange={(e) => patch({ graphqlCursorVariable: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pg-gqlpagesizevar">Page size variable</label>
            <input id="pg-gqlpagesizevar" className="input" placeholder="first" value={value?.graphqlPageSizeVariable ?? ""} onChange={(e) => patch({ graphqlPageSizeVariable: e.target.value })} />
          </div>
          <div className="field">
            <label htmlFor="pg-gqlpagesize">Page size</label>
            <input id="pg-gqlpagesize" className="input" type="number" value={value?.pageSize ?? 50} onChange={(e) => patch({ pageSize: Number(e.target.value) })} />
          </div>
        </div>
      )}

      {strategy !== "none" && (
        <div className="field">
          <label htmlFor="pg-maxpages">Max pages</label>
          <input
            id="pg-maxpages"
            className="input"
            type="number"
            min={1}
            max={500}
            placeholder="20"
            value={value?.maxPages ?? ""}
            onChange={(e) => patch({ maxPages: Number(e.target.value) || undefined })}
          />
          <span className="field-hint">Guardrail: fetching stops after this many pages even if more are available (hard ceiling 500).</span>
        </div>
      )}
    </>
  );
}
