import type { Folder } from "../api/types";
import { Select } from "./ui";

// Indented flat list (depth via em-dashes) rather than a popup tree picker -
// folder counts in this kind of app are expected to be small-to-moderate,
// so a plain <select> covers the "pick a folder" case without building a
// second tree-rendering surface just for pickers. See FolderTree for the
// full expand/collapse browsing experience on the Folders page itself.
export function folderOptionLabel(folder: Folder): string {
  return "—".repeat(folder.depth) + (folder.depth > 0 ? " " : "") + folder.name;
}

// Depth-first flatten (children immediately follow their parent, each level
// alphabetical) so the indentation in folderOptionLabel actually reads as a
// tree instead of a flat alphabetical list that happens to have dashes in it.
function flattenDepthFirst(folders: Folder[]): Folder[] {
  const children = new Map<string, Folder[]>();
  const roots: Folder[] = [];
  for (const f of folders) {
    if (f.parentId) {
      const list = children.get(f.parentId) ?? [];
      list.push(f);
      children.set(f.parentId, list);
    } else {
      roots.push(f);
    }
  }
  const byName = (a: Folder, b: Folder) => a.name.localeCompare(b.name);
  const out: Folder[] = [];
  function visit(list: Folder[]) {
    for (const f of [...list].sort(byName)) {
      out.push(f);
      visit(children.get(f.id) ?? []);
    }
  }
  visit(roots);
  return out;
}

interface FolderSelectProps {
  id: string;
  folders: Folder[];
  value: string;
  onChange: (folderId: string) => void;
  /** Folder id (and its descendants) to exclude - e.g. when picking a new
   * parent for a move, a folder can't become its own descendant. */
  excludeId?: string;
  placeholder?: string;
  disabled?: boolean;
}

export function FolderSelect({ id, folders, value, onChange, excludeId, placeholder, disabled }: FolderSelectProps) {
  const options = excludeId
    ? folders.filter((f) => f.id !== excludeId && !f.ancestorIds.includes(excludeId))
    : folders;
  const ordered = flattenDepthFirst(options);

  return (
    <Select id={id} value={value} onChange={(e) => onChange(e.target.value)} disabled={disabled}>
      {placeholder && <option value="">{placeholder}</option>}
      {ordered.map((f) => (
        <option key={f.id} value={f.id}>
          {folderOptionLabel(f)}
        </option>
      ))}
    </Select>
  );
}
