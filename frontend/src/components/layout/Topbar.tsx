import { useNavigate } from "react-router-dom";

import { useAuthStore } from "../../state/authStore";
import { ThemeSwitcher } from "../ThemeSwitcher";
import { IconLogout } from "../icons";

export function Topbar() {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();

  async function handleLogout() {
    await logout();
    navigate("/login", { replace: true });
  }

  return (
    <header className="layout-topbar">
      <div />
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <ThemeSwitcher />
        {user && (
          <>
            <span style={{ color: "var(--text-secondary)" }}>{user.displayName}</span>
            <button type="button" className="icon-btn" title="Log out" onClick={handleLogout}>
              <IconLogout width={15} height={15} />
            </button>
          </>
        )}
      </div>
    </header>
  );
}
