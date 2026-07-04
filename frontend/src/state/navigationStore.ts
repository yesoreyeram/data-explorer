import { create } from "zustand";

const FAVORITES_KEY = "de-nav-favorites";
const RECENTS_KEY = "de-nav-recents";
const MAX_RECENTS = 8;

export interface RecentRoute {
  href: string;
  title: string;
  visitedAt: string;
}

interface NavigationState {
  commandPaletteOpen: boolean;
  recentDrawerOpen: boolean;
  favorites: string[];
  recentRoutes: RecentRoute[];
  openCommandPalette: () => void;
  closeCommandPalette: () => void;
  openRecentDrawer: () => void;
  closeRecentDrawer: () => void;
  toggleFavorite: (href: string) => void;
  recordVisit: (href: string, title: string) => void;
}

function loadStringArray(key: string): string[] {
  try {
    const raw = localStorage.getItem(key);
    const parsed = raw ? (JSON.parse(raw) as unknown) : [];
    return Array.isArray(parsed) ? parsed.filter((value): value is string => typeof value === "string") : [];
  } catch {
    return [];
  }
}

function loadRecentRoutes(): RecentRoute[] {
  try {
    const raw = localStorage.getItem(RECENTS_KEY);
    const parsed = raw ? (JSON.parse(raw) as unknown) : [];
    return Array.isArray(parsed)
      ? parsed.filter(
          (value): value is RecentRoute =>
            typeof value === "object" &&
            value !== null &&
            typeof (value as RecentRoute).href === "string" &&
            typeof (value as RecentRoute).title === "string" &&
            typeof (value as RecentRoute).visitedAt === "string",
        )
      : [];
  } catch {
    return [];
  }
}

export const useNavigationStore = create<NavigationState>((set, get) => ({
  commandPaletteOpen: false,
  recentDrawerOpen: false,
  favorites: loadStringArray(FAVORITES_KEY),
  recentRoutes: loadRecentRoutes(),
  openCommandPalette: () => set({ commandPaletteOpen: true }),
  closeCommandPalette: () => set({ commandPaletteOpen: false }),
  openRecentDrawer: () => set({ recentDrawerOpen: true }),
  closeRecentDrawer: () => set({ recentDrawerOpen: false }),
  toggleFavorite: (href) => {
    const favorites = get().favorites.includes(href)
      ? get().favorites.filter((value) => value !== href)
      : [...get().favorites, href];
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites));
    set({ favorites });
  },
  recordVisit: (href, title) => {
    const next = [{ href, title, visitedAt: new Date().toISOString() }, ...get().recentRoutes.filter((item) => item.href !== href)].slice(0, MAX_RECENTS);
    localStorage.setItem(RECENTS_KEY, JSON.stringify(next));
    set({ recentRoutes: next });
  },
}));
