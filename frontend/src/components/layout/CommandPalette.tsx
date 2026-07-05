import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

import { searchWorkspace } from "../../api/search";
import type { SearchResult } from "../../api/types";
import { NAV_ITEMS } from "../../lib/navigation";
import { useAuthStore } from "../../state/authStore";
import { useNavigationStore } from "../../state/navigationStore";
import { Input, Kbd } from "../ui";

function resultKey(result: SearchResult) {
  return `${result.type}:${result.href}:${result.id ?? result.name}`;
}

export function CommandPalette() {
  const navigate = useNavigate();
  const hasPermission = useAuthStore((s) => s.hasPermission);
  const open = useNavigationStore((s) => s.commandPaletteOpen);
  const close = useNavigationStore((s) => s.closeCommandPalette);
  const recordVisit = useNavigationStore((s) => s.recordVisit);
  const favorites = useNavigationStore((s) => s.favorites);
  const recentRoutes = useNavigationStore((s) => s.recentRoutes);
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setSelectedIndex(0);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") close();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [close, open]);

  const allowedNavItems = useMemo(
    () => NAV_ITEMS.filter((item) => !item.permission || hasPermission(item.permission)),
    [hasPermission],
  );

  const searchQuery = useQuery({
    queryKey: ["workspace-search", query],
    queryFn: () => searchWorkspace(query),
    enabled: open && query.trim().length > 0,
    staleTime: 30_000,
  });

  const results = useMemo(() => {
    if (query.trim()) return searchQuery.data ?? [];
    const favoritesFirst = favorites
      .map((href) => allowedNavItems.find((item) => item.href === href))
      .filter((item): item is { href: string; title: string } => Boolean(item))
      .map((item) => ({ type: "favorite", href: item.href, name: item.title }));
    const recent = recentRoutes.map((item) => ({ type: "recent", href: item.href, name: item.title }));
    const pages = allowedNavItems.map((item) => ({ type: "page", href: item.href, name: item.title }));
    const deduped = new Map<string, SearchResult>();
    [...favoritesFirst, ...recent, ...pages].forEach((item) => deduped.set(`${item.type}:${item.href}:${item.name}`, item));
    return [...deduped.values()];
  }, [allowedNavItems, favorites, query, recentRoutes, searchQuery.data]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  function activate(result: SearchResult) {
    recordVisit(result.href, result.name);
    close();
    navigate(result.href);
  }

  if (!open) return null;

  return (
    <div className="overlay" role="presentation" onClick={close}>
      <div className="command-palette" role="dialog" aria-modal="true" aria-label="Command palette" onClick={(e) => e.stopPropagation()}>
        <div className="command-palette-header">
          <Input
            autoFocus
            className="command-palette-input"
            placeholder="Search pages, connections, and workflows"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedIndex((current) => Math.min(current + 1, Math.max(results.length - 1, 0)));
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedIndex((current) => Math.max(current - 1, 0));
              }
              if (event.key === "Enter" && results[selectedIndex]) {
                event.preventDefault();
                activate(results[selectedIndex]);
              }
            }}
          />
          <div className="command-palette-hint">
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd>
            <Kbd>Enter</Kbd>
            <Kbd>Esc</Kbd>
          </div>
        </div>
        <div className="command-palette-body">
          {searchQuery.isLoading ? (
            <div className="field-hint">Searching…</div>
          ) : results.length === 0 ? (
            <div className="empty-state">No matches</div>
          ) : (
            results.map((result, index) => (
              <button
                key={resultKey(result)}
                type="button"
                className={"command-result" + (index === selectedIndex ? " active" : "")}
                onMouseEnter={() => setSelectedIndex(index)}
                onClick={() => activate(result)}
              >
                <span>{result.name}</span>
                <span className="command-result-type">{result.type}</span>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
