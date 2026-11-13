CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE app_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'promoter', 'member')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Append-only application audit trail. Sensitive values must be redacted before
-- writing snapshots (for example, never include password hashes or KYC numbers).
CREATE TABLE audit_entries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_user_id UUID,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    request_id TEXT,
    before_state JSONB,
    after_state JSONB,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX audit_entries_entity_idx ON audit_entries (entity_type, entity_id, occurred_at DESC);
CREATE INDEX audit_entries_actor_idx ON audit_entries (actor_user_id, occurred_at DESC);

CREATE FUNCTION reject_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit entries are append-only';
END;
$$;

CREATE TRIGGER audit_entries_append_only
    BEFORE UPDATE OR DELETE ON audit_entries
    FOR EACH ROW EXECUTE FUNCTION reject_audit_mutation();

-- Transactional outbox: business state, audit entry, and event are committed together.
CREATE TABLE event_outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX event_outbox_pending_idx ON event_outbox (id) WHERE published_at IS NULL;
