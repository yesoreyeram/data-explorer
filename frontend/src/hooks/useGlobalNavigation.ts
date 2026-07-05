import { useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";

import { useNavigationStore } from "../state/navigationStore";

const GO_TO_SHORTCUTS: Record<string, string> = {
  d: "/",
  e: "/explore",
  c: "/connections",
  w: "/workflows",
  a: "/audit-log",
  u: "/users",
};

export function useGlobalNavigation() {
  const navigate = useNavigate();
  const openCommandPalette = useNavigationStore((s) => s.openCommandPalette);
  const openRecentDrawer = useNavigationStore((s) => s.openRecentDrawer);
  const pendingGoto = useRef(false);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const target = event.target;
      const isTypingTarget =
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement ||
        (target instanceof HTMLElement && target.isContentEditable);
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        openCommandPalette();
        return;
      }
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === ".") {
        event.preventDefault();
        openRecentDrawer();
        return;
      }
      if (isTypingTarget) return;
      if (pendingGoto.current) {
        pendingGoto.current = false;
        const destination = GO_TO_SHORTCUTS[event.key.toLowerCase()];
        if (destination) {
          event.preventDefault();
          navigate(destination);
        }
        return;
      }
      if (event.key.toLowerCase() === "g") {
        pendingGoto.current = true;
        window.setTimeout(() => {
          pendingGoto.current = false;
        }, 900);
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [navigate, openCommandPalette, openRecentDrawer]);

}
