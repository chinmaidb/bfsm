ALTER TABLE kyc_upload_sessions
    ADD COLUMN kyc_type TEXT CHECK (kyc_type IN ('aadhaar', 'pan', 'voter_id'));

ALTER TABLE kyc_documents
    ADD COLUMN kyc_type TEXT CHECK (kyc_type IN ('aadhaar', 'pan', 'voter_id'));

CREATE INDEX kyc_documents_member_type_idx ON kyc_documents(member_id, kyc_type);
