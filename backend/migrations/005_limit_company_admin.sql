-- A regular company administrator can manage accounts and role definitions,
-- but receives no operational permissions by default. Only the config-managed
-- super admin bypasses the permission grants.
DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM company_roles WHERE role_key = 'company_admin');

INSERT INTO role_permissions(role_id, permission)
SELECT id, permission
FROM company_roles
CROSS JOIN (VALUES
    ('accounts.read'), ('accounts.create'), ('accounts.update'), ('accounts.delete'),
    ('roles.read'), ('roles.create'), ('roles.update'), ('roles.delete'),
    ('audit.read')
) AS access_permissions(permission)
WHERE company_roles.role_key = 'company_admin';
