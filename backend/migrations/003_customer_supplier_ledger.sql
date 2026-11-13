CREATE TABLE parties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    party_type TEXT NOT NULL CHECK (party_type IN ('customer', 'supplier')),
    member_id UUID UNIQUE REFERENCES members(id),
    name TEXT NOT NULL,
    phone TEXT NOT NULL DEFAULT '',
    email TEXT,
    address TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (member_id IS NULL OR party_type = 'customer')
);
CREATE INDEX parties_type_name_idx ON parties (party_type, name);

CREATE TABLE financial_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_number BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    direction TEXT NOT NULL CHECK (direction IN ('receipt', 'payment')),
    party_id UUID NOT NULL REFERENCES parties(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    payment_method TEXT NOT NULL CHECK (payment_method IN ('cash', 'upi', 'bank_transfer', 'card', 'cheque', 'other')),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reference TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    recorded_by UUID REFERENCES app_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX financial_entries_party_date_idx ON financial_entries (party_id, occurred_at DESC);
CREATE INDEX financial_entries_direction_date_idx ON financial_entries (direction, occurred_at DESC);

CREATE FUNCTION validate_financial_party() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE expected_type TEXT;
BEGIN
    expected_type := CASE WHEN NEW.direction = 'receipt' THEN 'customer' ELSE 'supplier' END;
    IF NOT EXISTS (SELECT 1 FROM parties p WHERE p.id = NEW.party_id AND p.party_type = expected_type AND p.active) THEN
        RAISE EXCEPTION 'entry direction does not match an active party type';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER financial_entries_party_check
    BEFORE INSERT ON financial_entries
    FOR EACH ROW EXECUTE FUNCTION validate_financial_party();

CREATE TRIGGER financial_entries_immutable
    BEFORE UPDATE OR DELETE ON financial_entries
    FOR EACH ROW EXECUTE FUNCTION reject_financial_mutation();
