package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"schemeapp/backend/internal/security"
)

var permissionCatalog = map[string]string{
	"dashboard.read": "View dashboard",
	"promoters.read": "View promoters", "promoters.create": "Create promoters", "promoters.update": "Edit promoters", "promoters.delete": "Deactivate promoters",
	"members.read": "View members", "members.create": "Create members", "members.update": "Edit members", "members.delete": "Deactivate members",
	"schemes.read": "View schemes", "schemes.create": "Create schemes", "schemes.update": "Edit schemes", "schemes.delete": "Deactivate schemes",
	"enrollments.read": "View enrollments", "enrollments.create": "Create enrollments", "enrollments.update": "Change enrollment status",
	"enrollments.frequency.update": "Set own payment frequency",
	"payments.read": "View payments and receipts", "payments.create": "Record payments",
	"draws.read": "View draw results", "draws.create": "Record chosen members",
	"incentives.read": "View incentives", "incentives.create": "Create incentives", "incentives.update": "Update incentive status",
	"parties.read": "View customers and suppliers", "parties.create": "Create customers and suppliers", "parties.update": "Edit customers and suppliers", "parties.delete": "Deactivate customers and suppliers",
	"cash_receipts.read": "View customer receipts", "cash_receipts.create": "Record customer receipts",
	"supplier_payments.read": "View supplier payments", "supplier_payments.create": "Record supplier payments",
	"reports.read": "View reports", "audit.read": "View audit trail",
	"accounts.read": "View accounts", "accounts.create": "Create accounts", "accounts.update": "Edit accounts", "accounts.delete": "Deactivate accounts",
	"roles.read": "View roles", "roles.create": "Create roles", "roles.update": "Edit roles", "roles.delete": "Deactivate roles",
}

type AccessAccount struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	RoleID       string `json:"roleId"`
	RoleName     string `json:"roleName"`
	Active       bool   `json:"active"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
	CreatedAt    string `json:"createdAt"`
}

type AccountInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   string `json:"roleId"`
	Active   *bool  `json:"active"`
}

type CompanyRole struct {
	ID          string   `json:"id"`
	RoleKey     string   `json:"roleKey"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Active      bool     `json:"active"`
}

type RoleInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type Permission struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type UserAccess struct {
	Permissions  []string `json:"permissions"`
	IsSuperAdmin bool     `json:"isSuperAdmin"`
}

func Permissions() []Permission {
	out := make([]Permission, 0, len(permissionCatalog))
	for key, label := range permissionCatalog {
		out = append(out, Permission{Key: key, Label: label})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func (s *Service) UserAccess(ctx context.Context, userID string) (UserAccess, error) {
	access := UserAccess{Permissions: []string{}}
	var encoded []byte
	err := s.DB.QueryRowContext(ctx, `SELECT u.is_super_admin, COALESCE(jsonb_agg(rp.permission) FILTER (WHERE rp.permission IS NOT NULL), '[]'::jsonb)
		FROM app_users u
		LEFT JOIN company_roles r ON r.id=u.company_role_id AND r.active
		LEFT JOIN role_permissions rp ON rp.role_id=r.id
		WHERE u.id=$1 AND u.is_active
		GROUP BY u.id`, userID).Scan(&access.IsSuperAdmin, &encoded)
	if err != nil {
		return access, err
	}
	err = json.Unmarshal(encoded, &access.Permissions)
	return access, err
}

func (s *Service) Can(ctx context.Context, userID, permission string) (bool, error) {
	var allowed bool
	err := s.DB.QueryRowContext(ctx, `SELECT u.is_super_admin OR EXISTS (
		SELECT 1 FROM role_permissions rp JOIN company_roles r ON r.id=rp.role_id AND r.active
		WHERE rp.role_id=u.company_role_id AND rp.permission=$2
	) FROM app_users u WHERE u.id=$1 AND u.is_active`, userID, permission).Scan(&allowed)
	return allowed, err
}

func (s *Service) canAssignRole(ctx context.Context, actor Actor, roleID string) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT rp.permission FROM role_permissions rp JOIN company_roles r ON r.id=rp.role_id WHERE rp.role_id=$1 AND r.active`, roleID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return err
		}
		allowed, err := s.Can(ctx, actor.UserID, permission)
		if err != nil {
			return err
		}
		if !allowed {
			return invalid("you cannot assign a role with permissions you do not hold")
		}
	}
	return rows.Err()
}

func (s *Service) ListAccounts(ctx context.Context) ([]AccessAccount, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT u.id::text,u.username,u.role,COALESCE(r.id::text,''),CASE WHEN u.is_super_admin THEN 'Super admin (configuration)' ELSE COALESCE(r.name,u.role) END,u.is_active,u.is_super_admin,u.created_at::text
		FROM app_users u LEFT JOIN company_roles r ON r.id=u.company_role_id WHERE u.role IN ('admin','staff') ORDER BY u.created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AccessAccount, 0)
	for rows.Next() {
		var item AccessAccount
		if err := rows.Scan(&item.ID, &item.Username, &item.Role, &item.RoleID, &item.RoleName, &item.Active, &item.IsSuperAdmin, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) SaveAccount(ctx context.Context, actor Actor, id string, input AccountInput) (AccessAccount, error) {
	input.Username = strings.TrimSpace(input.Username)
	if len(input.Username) < 3 || len(input.Username) > 80 {
		return AccessAccount{}, invalid("username must be between 3 and 80 characters")
	}
	if input.RoleID == "" {
		return AccessAccount{}, invalid("roleId is required")
	}
	if len(input.Password) > 256 {
		return AccessAccount{}, invalid("password must not exceed 256 characters")
	}
	if id == "" && len(input.Password) < 12 {
		return AccessAccount{}, invalid("password must be at least 12 characters")
	}
	var hash string
	if input.Password != "" {
		if len(input.Password) < 12 {
			return AccessAccount{}, invalid("password must be at least 12 characters")
		}
		var err error
		hash, err = security.HashPassword(input.Password)
		if err != nil {
			return AccessAccount{}, err
		}
	}
	if err := s.canAssignRole(ctx, actor, input.RoleID); err != nil {
		return AccessAccount{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AccessAccount{}, err
	}
	defer tx.Rollback()
	var before []byte
	if id != "" {
		err = tx.QueryRowContext(ctx, `SELECT jsonb_build_object('id',u.id,'username',u.username,'role',u.role,'roleId',u.company_role_id,'active',u.is_active,'isSuperAdmin',u.is_super_admin) FROM app_users u WHERE u.id=$1 FOR UPDATE`, id).Scan(&before)
		if err != nil {
			return AccessAccount{}, err
		}
		var protected bool
		if err = tx.QueryRowContext(ctx, `SELECT is_super_admin OR role NOT IN ('staff','admin') FROM app_users WHERE id=$1`, id).Scan(&protected); err != nil {
			return AccessAccount{}, err
		}
		if protected {
			return AccessAccount{}, invalid("this account is managed by system configuration")
		}
	}
	var roleActive bool
	if err = tx.QueryRowContext(ctx, `SELECT active FROM company_roles WHERE id=$1`, input.RoleID).Scan(&roleActive); err != nil {
		return AccessAccount{}, err
	}
	if !roleActive {
		return AccessAccount{}, invalid("choose an active company role")
	}
	var accountID string
	if id == "" {
		if hash == "" {
			return AccessAccount{}, invalid("password is required")
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO app_users(username,password_hash,role,company_role_id) VALUES($1,$2,'staff',$3) RETURNING id::text`, input.Username, hash, input.RoleID).Scan(&accountID)
	} else {
		accountID = id
		if hash == "" {
			_, err = tx.ExecContext(ctx, `UPDATE app_users SET username=$2,company_role_id=$3,is_active=COALESCE($4,is_active),updated_at=now() WHERE id=$1 AND role IN ('staff','admin') AND NOT is_super_admin`, id, input.Username, input.RoleID, input.Active)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE app_users SET username=$2,company_role_id=$3,password_hash=$4,is_active=COALESCE($5,is_active),updated_at=now() WHERE id=$1 AND role IN ('staff','admin') AND NOT is_super_admin`, id, input.Username, input.RoleID, hash, input.Active)
		}
	}
	if err != nil {
		return AccessAccount{}, err
	}
	action := "created"
	if id != "" {
		action = "updated"
	}
	var after []byte
	if err = tx.QueryRowContext(ctx, `SELECT jsonb_build_object('id',u.id,'username',u.username,'role',u.role,'roleId',r.id,'roleName',r.name,'active',u.is_active,'isSuperAdmin',u.is_super_admin) FROM app_users u JOIN company_roles r ON r.id=u.company_role_id WHERE u.id=$1`, accountID).Scan(&after); err != nil {
		return AccessAccount{}, err
	}
	if err = writeAudit(ctx, tx, actor, action, "app_users", accountID, before, after); err != nil {
		return AccessAccount{}, err
	}
	if err = writeEvent(ctx, tx, "identity.account_"+action+".v1", "app_users", accountID, after); err != nil {
		return AccessAccount{}, err
	}
	if err = tx.Commit(); err != nil {
		return AccessAccount{}, err
	}
	var out AccessAccount
	err = json.Unmarshal(after, &out)
	return out, err
}

func (s *Service) DeleteAccount(ctx context.Context, actor Actor, id string) error {
	if id == actor.UserID {
		return invalid("you cannot deactivate your own account")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before, after []byte
	if err = tx.QueryRowContext(ctx, `SELECT jsonb_build_object('id',id,'username',username,'role',role,'roleId',company_role_id,'active',is_active,'isSuperAdmin',is_super_admin) FROM app_users WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
		return err
	}
	var protected bool
	if err = tx.QueryRowContext(ctx, `SELECT is_super_admin OR role NOT IN ('staff','admin') FROM app_users WHERE id=$1`, id).Scan(&protected); err != nil {
		return err
	}
	if protected {
		return invalid("this account is managed by system configuration")
	}
	if err = tx.QueryRowContext(ctx, `UPDATE app_users SET is_active=FALSE,updated_at=now() WHERE id=$1 RETURNING jsonb_build_object('id',id,'username',username,'role',role,'roleId',company_role_id,'active',is_active,'isSuperAdmin',is_super_admin)`, id).Scan(&after); err != nil {
		return err
	}
	if err = writeAudit(ctx, tx, actor, "deleted", "app_users", id, before, after); err != nil {
		return err
	}
	if err = writeEvent(ctx, tx, "identity.account_deleted.v1", "app_users", id, after); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) ListCompanyRoles(ctx context.Context) ([]CompanyRole, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT r.id::text,r.role_key,r.name,r.description,r.active,COALESCE(jsonb_agg(rp.permission) FILTER (WHERE rp.permission IS NOT NULL),'[]'::jsonb) FROM company_roles r LEFT JOIN role_permissions rp ON rp.role_id=r.id GROUP BY r.id ORDER BY r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CompanyRole, 0)
	for rows.Next() {
		var item CompanyRole
		var encoded []byte
		if err = rows.Scan(&item.ID, &item.RoleKey, &item.Name, &item.Description, &item.Active, &encoded); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(encoded, &item.Permissions); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

var roleSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func roleSlug(name string) string {
	return strings.Trim(roleSlugPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "_"), "_")
}

func (s *Service) SaveCompanyRole(ctx context.Context, actor Actor, id string, input RoleInput) (CompanyRole, error) {
	input.Name = strings.TrimSpace(input.Name)
	if len(input.Name) < 2 || len(input.Name) > 80 {
		return CompanyRole{}, invalid("role name must be between 2 and 80 characters")
	}
	if len(input.Description) > 500 {
		return CompanyRole{}, invalid("description is too long")
	}
	for _, permission := range input.Permissions {
		if _, ok := permissionCatalog[permission]; !ok {
			return CompanyRole{}, invalid("unknown permission: " + permission)
		}
	}
	for _, permission := range input.Permissions {
		allowed, err := s.Can(ctx, actor.UserID, permission)
		if err != nil {
			return CompanyRole{}, err
		}
		if !allowed {
			return CompanyRole{}, invalid("you cannot grant permissions you do not hold")
		}
	}
	sort.Strings(input.Permissions)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return CompanyRole{}, err
	}
	defer tx.Rollback()
	var before []byte
	created := id == ""
	if id != "" {
		if err = tx.QueryRowContext(ctx, `SELECT jsonb_build_object('id',r.id,'roleKey',r.role_key,'name',r.name,'description',r.description,'active',r.active,'permissions',COALESCE((SELECT jsonb_agg(permission ORDER BY permission) FROM role_permissions WHERE role_id=r.id),'[]'::jsonb)) FROM company_roles r WHERE r.id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
			return CompanyRole{}, err
		}
		var key string
		if err = tx.QueryRowContext(ctx, `SELECT role_key FROM company_roles WHERE id=$1`, id).Scan(&key); err != nil {
			return CompanyRole{}, err
		}
		if key == "company_admin" {
			return CompanyRole{}, invalid("the built-in administrator role is protected")
		}
	} else {
		key := roleSlug(input.Name)
		if key == "" {
			return CompanyRole{}, invalid("role name must contain letters or numbers")
		}
		var insertedID string
		if err = tx.QueryRowContext(ctx, `INSERT INTO company_roles(role_key,name,description) VALUES($1,$2,$3) RETURNING id::text`, key, input.Name, input.Description).Scan(&insertedID); err != nil {
			return CompanyRole{}, err
		}
		id = insertedID
	}
	if id != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE company_roles SET name=$2,description=$3,active=TRUE,updated_at=now() WHERE id=$1`, id, input.Name, input.Description); err != nil {
			return CompanyRole{}, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, id); err != nil {
			return CompanyRole{}, err
		}
		for _, permission := range input.Permissions {
			if _, err = tx.ExecContext(ctx, `INSERT INTO role_permissions(role_id,permission) VALUES($1,$2)`, id, permission); err != nil {
				return CompanyRole{}, err
			}
		}
	}
	var after []byte
	if err = tx.QueryRowContext(ctx, `SELECT jsonb_build_object('id',r.id,'roleKey',r.role_key,'name',r.name,'description',r.description,'active',r.active,'permissions',COALESCE((SELECT jsonb_agg(permission ORDER BY permission) FROM role_permissions WHERE role_id=r.id),'[]'::jsonb)) FROM company_roles r WHERE r.id=$1`, id).Scan(&after); err != nil {
		return CompanyRole{}, err
	}
	action := "updated"
	if created {
		action = "created"
	}
	if err = writeAudit(ctx, tx, actor, action, "company_roles", id, before, after); err != nil {
		return CompanyRole{}, err
	}
	if err = writeEvent(ctx, tx, "access.role_"+action+".v1", "company_roles", id, after); err != nil {
		return CompanyRole{}, err
	}
	if err = tx.Commit(); err != nil {
		return CompanyRole{}, err
	}
	var result CompanyRole
	err = json.Unmarshal(after, &result)
	return result, err
}

func (s *Service) DeleteCompanyRole(ctx context.Context, actor Actor, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before, after []byte
	var roleKey string
	if err = tx.QueryRowContext(ctx, `SELECT role_key,jsonb_build_object('id',r.id,'roleKey',r.role_key,'name',r.name,'description',r.description,'active',r.active,'permissions',COALESCE((SELECT jsonb_agg(permission ORDER BY permission) FROM role_permissions WHERE role_id=r.id),'[]'::jsonb)) FROM company_roles r WHERE id=$1 FOR UPDATE`, id).Scan(&roleKey, &before); err != nil {
		return err
	}
	if roleKey == "company_admin" {
		return invalid("the built-in administrator role is protected")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE company_roles SET active=FALSE,updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT jsonb_build_object('id',r.id,'roleKey',r.role_key,'name',r.name,'description',r.description,'active',r.active,'permissions',COALESCE((SELECT jsonb_agg(permission ORDER BY permission) FROM role_permissions WHERE role_id=r.id),'[]'::jsonb)) FROM company_roles r WHERE r.id=$1`, id).Scan(&after); err != nil {
		return err
	}
	if err = writeAudit(ctx, tx, actor, "deleted", "company_roles", id, before, after); err != nil {
		return err
	}
	if err = writeEvent(ctx, tx, "access.role_deleted.v1", "company_roles", id, after); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) EnsureSuperAdmin(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 80 || len(password) < 16 || len(password) > 256 {
		return invalid("SUPER_ADMIN_USERNAME must be 3-80 characters and SUPER_ADMIN_PASSWORD 16-256 characters")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before []byte
	var existingHash, oldRole string
	var wasActive, wasSuper, hasAdminRole bool
	err = tx.QueryRowContext(ctx, `SELECT jsonb_build_object('id',id,'username',username,'role',role,'active',is_active,'isSuperAdmin',is_super_admin),password_hash,role,is_active,is_super_admin,COALESCE(company_role_id=(SELECT id FROM company_roles WHERE role_key='company_admin'),FALSE) FROM app_users WHERE username=$1 FOR UPDATE`, username).Scan(&before, &existingHash, &oldRole, &wasActive, &wasSuper, &hasAdminRole)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		hash, hashErr := security.HashPassword(password)
		if hashErr != nil {
			return hashErr
		}
		var createdID string
		err = tx.QueryRowContext(ctx, `INSERT INTO app_users(username,password_hash,role,is_super_admin,company_role_id) SELECT $1,$2,'admin',TRUE,id FROM company_roles WHERE role_key='company_admin' RETURNING id::text`, username, hash).Scan(&createdID)
		if err == sql.ErrNoRows {
			return fmt.Errorf("company_admin role is missing; apply database migrations through 004_access_control.sql")
		}
	} else {
		if oldRole == "member" || oldRole == "promoter" {
			return invalid("configured super admin username is already assigned to a member or promoter account")
		}
		matches, verifyErr := security.VerifyPassword(existingHash, password)
		if verifyErr == nil && matches && oldRole == "admin" && wasActive && wasSuper && hasAdminRole {
			return tx.Rollback()
		}
		hash, hashErr := security.HashPassword(password)
		if hashErr != nil {
			return hashErr
		}
		_, err = tx.ExecContext(ctx, `UPDATE app_users SET password_hash=$2,role='admin',is_active=TRUE,is_super_admin=TRUE,company_role_id=(SELECT id FROM company_roles WHERE role_key='company_admin'),updated_at=now() WHERE username=$1`, username, hash)
	}
	if err != nil {
		return err
	}
	var id string
	var after []byte
	if err = tx.QueryRowContext(ctx, `SELECT id::text,jsonb_build_object('id',id,'username',username,'role',role,'active',is_active,'isSuperAdmin',is_super_admin) FROM app_users WHERE username=$1`, username).Scan(&id, &after); err != nil {
		return err
	}
	action := "configured"
	if !json.Valid(before) {
		action = "created"
	}
	if err = writeAudit(ctx, tx, Actor{Role: "system"}, "super_admin_"+action, "app_users", id, before, after); err != nil {
		return err
	}
	if err = writeEvent(ctx, tx, "identity.super_admin_"+action+".v1", "app_users", id, after); err != nil {
		return err
	}
	return tx.Commit()
}
