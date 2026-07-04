import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Modal } from "../../components/Modal";
import { listCatalog } from "../../api/catalog";
import type { CatalogEntry } from "../../api/types";
import { Badge, Input, Select } from "../../components/ui";

interface CatalogBrowserModalProps {
  onClose: () => void;
  onSelect: (entry: CatalogEntry) => void;
}

export function CatalogBrowserModal({ onClose, onSelect }: CatalogBrowserModalProps) {
  const { data: entries = [], isLoading } = useQuery({ queryKey: ["catalog"], queryFn: () => listCatalog() });
  const [q, setQ] = useState("");
  const [type, setType] = useState<"" | "rest" | "graphql">("");
  const [category, setCategory] = useState("");

  const categories = useMemo(() => Array.from(new Set(entries.map((e) => e.category))).sort(), [entries]);

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return entries.filter((e) => {
      if (type && e.type !== type) return false;
      if (category && e.category !== category) return false;
      if (
        needle &&
        !e.name.toLowerCase().includes(needle) &&
        !e.description.toLowerCase().includes(needle) &&
        !e.category.toLowerCase().includes(needle)
      ) {
        return false;
      }
      return true;
    });
  }, [entries, q, type, category]);

  return (
    <Modal title="Browse integration catalog" onClose={onClose} width={640}>
      <p className="field-hint" style={{ marginBottom: 12 }}>
        Pick a well-known integration to prefill a new connection's base URL and authentication - you'll still supply
        your own credentials.
      </p>
      <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr 1fr", gap: 8, marginBottom: 12 }}>
        <Input placeholder="Search integrations..." value={q} onChange={(e) => setQ(e.target.value)} autoFocus />
        <Select value={type} onChange={(e) => setType(e.target.value as "" | "rest" | "graphql")}>
          <option value="">All types</option>
          <option value="rest">REST</option>
          <option value="graphql">GraphQL</option>
        </Select>
        <Select value={category} onChange={(e) => setCategory(e.target.value)}>
          <option value="">All categories</option>
          {categories.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </Select>
      </div>

      {isLoading && <p className="field-hint">Loading catalog...</p>}

      <div style={{ maxHeight: 420, overflowY: "auto", display: "flex", flexDirection: "column", gap: 6 }}>
        {filtered.map((e) => (
          <button
            key={e.id}
            type="button"
            onClick={() => onSelect(e)}
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              gap: 12,
              textAlign: "left",
              padding: "10px 12px",
              border: "1px solid var(--border)",
              borderRadius: 8,
              background: "transparent",
              cursor: "pointer",
            }}
          >
            <div>
              <div style={{ fontWeight: 600 }}>{e.name}</div>
              <p className="field-hint" style={{ margin: 0 }}>
                {e.description}
              </p>
            </div>
            <div style={{ display: "flex", gap: 6, flexShrink: 0 }}>
              <Badge>{e.category}</Badge>
              <Badge>{e.type}</Badge>
            </div>
          </button>
        ))}
        {!isLoading && filtered.length === 0 && <p className="field-hint">No integrations match your search.</p>}
      </div>
    </Modal>
  );
}
