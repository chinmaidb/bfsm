INSERT INTO company_roles(role_key, name, description)
VALUES
    ('company_admin', 'Company administrator', 'Company-level administration role'),
    ('promoter', 'Promoter', 'Company promoter portal role'),
    ('customer', 'Customer', 'Company customer portal role')
ON CONFLICT (role_key) DO NOTHING;

-- Company portal roles receive only the operations needed in their own portal.
INSERT INTO role_permissions(role_id, permission)
SELECT r.id, p.permission
FROM company_roles r
JOIN (VALUES
    ('promoter', 'dashboard.read'),
    ('promoter', 'members.read'), ('promoter', 'members.create'),
    ('promoter', 'enrollments.read'), ('promoter', 'enrollments.create'),
    ('promoter', 'payments.read'), ('promoter', 'payments.create'),
    ('promoter', 'incentives.read'),
    ('customer', 'dashboard.read'), ('customer', 'enrollments.read'),
    ('customer', 'enrollments.frequency.update'),
    ('customer', 'payments.read')
) AS p(role_key, permission) ON p.role_key = r.role_key
ON CONFLICT (role_id, permission) DO NOTHING;

-- Link existing portal accounts to their matching role so auth/me and the
-- role-filtered navigation return the same grants as newly created accounts.
UPDATE app_users u
SET company_role_id = r.id
FROM company_roles r
WHERE u.company_role_id IS NULL
  AND ((u.role = 'promoter' AND r.role_key = 'promoter')
       OR (u.role = 'member' AND r.role_key = 'customer'));
