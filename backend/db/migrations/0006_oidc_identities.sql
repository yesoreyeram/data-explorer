-- Support single sign-on via OIDC alongside local email+password.

-- OIDC users have no local password.
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- One row per external identity linked to a local user. A user may link
-- several providers; (issuer, subject) is globally unique.
CREATE TABLE federated_identities (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer     TEXT NOT NULL,
    subject    TEXT NOT NULL,
    email      CITEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);

CREATE INDEX idx_federated_identities_user_id ON federated_identities(user_id);
