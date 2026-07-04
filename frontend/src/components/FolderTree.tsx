import { useEffect, useMemo, useState } from "react";

import type { Folder } from "../api/types";
import { IconButton } from "./ui";
import { IconChevronDown, IconChevronRight, IconFolder } from "./icons";

interface FolderTreeProps {
  folders: Folder[];
  selectedId?: string;
  onSelect: (folder: Folder) => void;
}

// The only tree/recursive UI in this frontend - built from scratch (no
// existing tree component anywhere in the app) using plain nested <div>s and
// a Set<string> of expanded folder ids, rather than pulling in a tree-view
// library for a single, fairly small use case.
export function FolderTree({ folders, selectedId, onSelect }: FolderTreeProps) {
  const childrenByParent = useMemo(() => {
    const map = new Map<string, Folder[]>();
    for (const f of folders) {
      const key = f.parentId ?? "";
      const list = map.get(key) ?? [];
      list.push(f);
      map.set(key, list);
    }
    for (const list of map.values()) list.sort((a, b) => a.name.localeCompare(b.name));
    return map;
  }, [folders]);

  // Expand every ancestor of the selected folder so navigating to a deep
  // folder (creating a subfolder, following a "move" action, ...) never
  // leaves it hidden behind collapsed parents - re-runs whenever the
  // selection changes, not just on mount, since e.g. creating a new
  // subfolder selects it well after this component first rendered.
  const [expanded, setExpanded] = useState<Set<string>>(() => {
    const selected = folders.find((f) => f.id === selectedId);
    return new Set(selected?.ancestorIds ?? []);
  });

  useEffect(() => {
    const selected = folders.find((f) => f.id === selectedId);
    if (!selected || selected.ancestorIds.length === 0) return;
    setExpanded((prev) => {
      const missing = selected.ancestorIds.filter((id) => !prev.has(id));
      if (missing.length === 0) return prev;
      return new Set([...prev, ...missing]);
    });
  }, [selectedId, folders]);

  function toggle(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  const roots = childrenByParent.get("") ?? [];
  if (roots.length === 0) {
    return <p className="empty-state">No folders yet.</p>;
  }

  return (
    <div className="folder-tree">
      {roots.map((f) => (
        <FolderTreeNode
          key={f.id}
          folder={f}
          depth={0}
          childrenByParent={childrenByParent}
          expanded={expanded}
          onToggle={toggle}
          selectedId={selectedId}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}

function FolderTreeNode({
  folder,
  depth,
  childrenByParent,
  expanded,
  onToggle,
  selectedId,
  onSelect,
}: {
  folder: Folder;
  depth: number;
  childrenByParent: Map<string, Folder[]>;
  expanded: Set<string>;
  onToggle: (id: string) => void;
  selectedId?: string;
  onSelect: (folder: Folder) => void;
}) {
  const children = childrenByParent.get(folder.id) ?? [];
  const isExpanded = expanded.has(folder.id);
  const isSelected = folder.id === selectedId;

  return (
    <div>
      <div
        className={"folder-tree-row" + (isSelected ? " selected" : "")}
        style={{ paddingLeft: depth * 16 }}
        onClick={() => onSelect(folder)}
      >
        {children.length > 0 ? (
          <IconButton
            label={isExpanded ? "Collapse" : "Expand"}
            onClick={(e) => {
              e.stopPropagation();
              onToggle(folder.id);
            }}
          >
            {isExpanded ? <IconChevronDown width={12} height={12} /> : <IconChevronRight width={12} height={12} />}
          </IconButton>
        ) : (
          <span style={{ display: "inline-block", width: 22 }} />
        )}
        <IconFolder width={14} height={14} />
        <span className="folder-tree-name">{folder.name}</span>
      </div>
      {isExpanded &&
        children.map((child) => (
          <FolderTreeNode
            key={child.id}
            folder={child}
            depth={depth + 1}
            childrenByParent={childrenByParent}
            expanded={expanded}
            onToggle={onToggle}
            selectedId={selectedId}
            onSelect={onSelect}
          />
        ))}
    </div>
  );
}
