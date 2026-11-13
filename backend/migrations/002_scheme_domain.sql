CREATE TABLE promoters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE REFERENCES app_users(id),
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    email TEXT,
    address TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE REFERENCES app_users(id),
    promoter_id UUID NOT NULL REFERENCES promoters(id),
    member_number BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    email TEXT,
    address TEXT NOT NULL DEFAULT '',
    kyc_type TEXT CHECK (kyc_type IN ('aadhaar', 'pan', 'voter_id')),
    kyc_last4 TEXT,
    kyc_verified_at TIMESTAMPTZ,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX members_promoter_idx ON members (promoter_id, name);

CREATE TABLE schemes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    contribution_minor BIGINT NOT NULL CHECK (contribution_minor > 0),
    duration_months INTEGER NOT NULL CHECK (duration_months > 0),
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id UUID NOT NULL REFERENCES members(id),
    scheme_id UUID NOT NULL REFERENCES schemes(id),
    start_month DATE NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'paused', 'completed', 'cancelled')) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (member_id, scheme_id, start_month)
);
CREATE INDEX enrollments_scheme_idx ON enrollments (scheme_id, status);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receipt_number BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    member_id UUID NOT NULL REFERENCES members(id),
    enrollment_id UUID NOT NULL REFERENCES enrollments(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    paid_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payment_method TEXT NOT NULL CHECK (payment_method IN ('cash', 'upi', 'bank_transfer', 'card', 'other')),
    reference TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    received_by UUID REFERENCES app_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX payments_member_date_idx ON payments (member_id, paid_at DESC);
CREATE INDEX payments_date_idx ON payments (paid_at DESC);

CREATE TABLE draw_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scheme_id UUID NOT NULL REFERENCES schemes(id),
    draw_month DATE NOT NULL,
    member_id UUID NOT NULL REFERENCES members(id),
    chosen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    recorded_by UUID REFERENCES app_users(id),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (scheme_id, draw_month)
);

CREATE TABLE promoter_incentives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promoter_id UUID NOT NULL REFERENCES promoters(id),
    period DATE NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    commission_minor BIGINT NOT NULL DEFAULT 0 CHECK (commission_minor >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('draft', 'approved', 'paid')) DEFAULT 'draft',
    created_by UUID REFERENCES app_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX promoter_incentives_period_idx ON promoter_incentives (promoter_id, period DESC);

-- Financial and draw records are corrected by compensating entries, never edits.
CREATE FUNCTION reject_financial_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'financial and draw records are immutable; record a correction instead';
END;
$$;
CREATE TRIGGER payments_immutable BEFORE UPDATE OR DELETE ON payments FOR EACH ROW EXECUTE FUNCTION reject_financial_mutation();
CREATE TRIGGER draw_results_immutable BEFORE UPDATE OR DELETE ON draw_results FOR EACH ROW EXECUTE FUNCTION reject_financial_mutation();
