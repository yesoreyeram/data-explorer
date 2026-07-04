import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";

import { extractErrorMessage } from "../api/client";
import { useAuthStore } from "../state/authStore";
import { Button, Card, CardBody, Field, Input } from "../components/ui";

export function RegisterPage() {
  const registerUser = useAuthStore((s) => s.register);
  const navigate = useNavigate();

  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await registerUser(email, displayName, password);
      navigate("/", { replace: true });
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-shell">
      <Card className="auth-card">
        <CardBody>
          <h1 className="panel-title">Create account</h1>
          <p className="panel-subtitle">New accounts start with read-only viewer access.</p>

          {error && <div className="error-banner">{error}</div>}

          <form onSubmit={handleSubmit}>
            <Field htmlFor="displayName" label="Full name">
              <Input id="displayName" required value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
            </Field>
            <Field htmlFor="email" label="Email">
              <Input
                id="email"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </Field>
            <Field htmlFor="password" label="Password" hint="At least 12 characters.">
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                required
                minLength={12}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </Field>
            <Button variant="primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
              {submitting ? "Creating account..." : "Create account"}
            </Button>
          </form>

          <p className="field-hint" style={{ marginTop: 12, textAlign: "center" }}>
            Already have an account? <Link to="/login">Sign in</Link>
          </p>
        </CardBody>
      </Card>
    </div>
  );
}
