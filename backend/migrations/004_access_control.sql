ALTER TABLE app_users DROP CONSTRAINT app_users_role_check;
ALTER TABLE app_users ADD CONSTRAINT app_users_role_check CHECK (role IN ('admin', 'promoter', 'member', 'staff'));
ALTER TABLE app_users ADD COLUMN is_super_admin BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE company_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES company_roles(id),
    permission TEXT NOT NULL,
    PRIMARY KEY (role_id, permission)
);

ALTER TABLE app_users ADD COLUMN company_role_id UUID REFERENCES company_roles(id);
CREATE INDEX app_users_company_role_idx ON app_users(company_role_id) WHERE company_role_id IS NOT NULL;

INSERT INTO company_roles(role_key, name, description)
VALUES ('company_admin', 'Company administrator', 'Full access to company operations and access control');

INSERT INTO role_permissions(role_id, permission)
SELECT id, permission
FROM company_roles
CROSS JOIN (VALUES
    ('dashboard.read'),
    ('promoters.read'), ('promoters.create'), ('promoters.update'), ('promoters.delete'),
    ('members.read'), ('members.create'), ('members.update'), ('members.delete'),
    ('schemes.read'), ('schemes.create'), ('schemes.update'), ('schemes.delete'),
    ('enrollments.read'), ('enrollments.create'), ('enrollments.update'),
    ('payments.read'), ('payments.create'),
    ('draws.read'), ('draws.create'),
    ('incentives.read'), ('incentives.create'), ('incentives.update'),
    ('parties.read'), ('parties.create'), ('parties.update'), ('parties.delete'),
    ('cash_receipts.read'), ('cash_receipts.create'),
    ('supplier_payments.read'), ('supplier_payments.create'),
    ('reports.read'), ('audit.read'),
    ('accounts.read'), ('accounts.create'), ('accounts.update'), ('accounts.delete'),
    ('roles.read'), ('roles.create'), ('roles.update'), ('roles.delete')
) AS permission_catalog(permission)
WHERE company_roles.role_key = 'company_admin';

UPDATE app_users
SET company_role_id = (SELECT id FROM company_roles WHERE role_key = 'company_admin')
WHERE role = 'admin';
