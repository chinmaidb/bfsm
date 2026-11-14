CREATE TABLE kyc_upload_sessions (
    slug TEXT PRIMARY KEY,
    member_id UUID NOT NULL REFERENCES members(id),
    upload_id TEXT NOT NULL,
    object_key TEXT NOT NULL,
    title TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size > 0),
    part_size BIGINT NOT NULL,
    initiated_by UUID REFERENCES app_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE kyc_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    member_id UUID NOT NULL REFERENCES members(id),
    title TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size > 0),
    object_key TEXT NOT NULL UNIQUE,
    uploaded_by UUID REFERENCES app_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX kyc_documents_member_created_idx ON kyc_documents(member_id, created_at DESC);
