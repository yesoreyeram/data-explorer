import { useThemeStore, type ThemeMode } from "../state/themeStore";
import { IconMonitor, IconMoon, IconSun } from "./icons";

const OPTIONS: { mode: ThemeMode; label: string; Icon: typeof IconSun }[] = [
  { mode: "light", label: "Light theme", Icon: IconSun },
  { mode: "dark", label: "Dark theme", Icon: IconMoon },
  { mode: "system", label: "Match system theme", Icon: IconMonitor },
];

export function ThemeSwitcher() {
  const { mode, setMode } = useThemeStore();

  return (
    <div className="theme-switcher" role="group" aria-label="Theme">
      {OPTIONS.map(({ mode: optionMode, label, Icon }) => (
        <button
          key={optionMode}
          type="button"
          className={optionMode === mode ? "active" : ""}
          aria-label={label}
          aria-pressed={optionMode === mode}
          title={label}
          onClick={() => setMode(optionMode)}
        >
          <Icon width={14} height={14} />
        </button>
      ))}
    </div>
  );
}
