import { useState, type FormEvent } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";

import { extractErrorMessage } from "../api/client";
import { useAuthStore } from "../state/authStore";

export function LoginPage() {
  const login = useAuthStore((s) => s.login);
  const navigate = useNavigate();
  const location = useLocation();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await login(email, password);
      const from = (location.state as { from?: { pathname?: string } } | null)?.from;
      navigate(from?.pathname ?? "/", { replace: true });
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-shell">
      <div className="card auth-card">
        <div className="card-body">
          <h1 className="panel-title">Sign in</h1>
          <p className="panel-subtitle">Data Explorer &mdash; connect, transform, explore.</p>

          {error && <div className="error-banner">{error}</div>}

          <form onSubmit={handleSubmit}>
            <div className="field">
              <label htmlFor="email">Email</label>
              <input
                id="email"
                className="input"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="field">
              <label htmlFor="password">Password</label>
              <input
                id="password"
                className="input"
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <button className="btn btn-primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
              {submitting ? "Signing in..." : "Sign in"}
            </button>
          </form>

          <p className="field-hint" style={{ marginTop: 12, textAlign: "center" }}>
            No account? <Link to="/register">Create one</Link>
          </p>
        </div>
      </div>
    </div>
  );
}
