package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func camelJSON(raw []byte) []byte {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return raw
	}
	var convert func(any) any
	convert = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := make(map[string]any, len(x))
			for key, val := range x {
				out[camelKey(key)] = convert(val)
			}
			return out
		case []any:
			out := make([]any, len(x))
			for i, val := range x {
				out[i] = convert(val)
			}
			return out
		default:
			return v
		}
	}
	result, err := json.Marshal(convert(value))
	if err != nil {
		return raw
	}
	return result
}
func camelKey(value string) string {
	var out strings.Builder
	upper := false
	for _, r := range value {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			if r >= 'a' && r <= 'z' {
				r -= 32
			}
			upper = false
		}
		out.WriteRune(r)
	}
	return out.String()
}

type Service struct{ DB *sql.DB }

type InputError string

func (e InputError) Error() string { return string(e) }
func invalid(message string) error { return InputError(message) }
func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func (s *Service) Login(ctx context.Context, username string) (string, string, string, error) {
	var id, hash, role string
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,password_hash,role FROM app_users WHERE username=$1 AND is_active`, strings.TrimSpace(username)).Scan(&id, &hash, &role)
	return id, hash, role, err
}

func (s *Service) List(ctx context.Context, kind string, limit, offset int) ([]json.RawMessage, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	queries := map[string]string{
		"promoters": `SELECT to_jsonb(p) FROM promoters p ORDER BY name LIMIT $1 OFFSET $2`,
		"members":   `SELECT to_jsonb(m) FROM members m ORDER BY member_number LIMIT $1 OFFSET $2`,
		"schemes":   `SELECT to_jsonb(s) FROM schemes s ORDER BY name LIMIT $1 OFFSET $2`,
	}
	query, ok := queries[kind]
	if !ok {
		return nil, invalid("unknown resource")
	}
	rows, err := s.DB.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) ListParties(ctx context.Context, partyType string) ([]json.RawMessage, error) {
	if partyType != "" && partyType != "customer" && partyType != "supplier" {
		return nil, invalid("type must be customer or supplier")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(p) FROM parties p WHERE ($1='' OR p.party_type=$1) ORDER BY p.name LIMIT 500`, partyType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) SaveParty(ctx context.Context, actor Actor, id string, input json.RawMessage) (json.RawMessage, error) {
	var v Party
	if err := json.Unmarshal(input, &v); err != nil {
		return nil, invalid("invalid party data")
	}
	if v.PartyType != "customer" && v.PartyType != "supplier" {
		return nil, invalid("partyType must be customer or supplier")
	}
	if strings.TrimSpace(v.Name) == "" {
		return nil, invalid("name is required")
	}
	if v.MemberID != "" && v.PartyType != "customer" {
		return nil, invalid("only a customer can be linked to a member")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var before, after []byte
	if id != "" {
		if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(p) FROM parties p WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
			return nil, err
		}
		var snapshot map[string]any
		if err = json.Unmarshal(before, &snapshot); err != nil {
			return nil, err
		}
		existingType, _ := snapshot["party_type"].(string)
		if existingType != v.PartyType {
			return nil, invalid("party type cannot be changed after creation")
		}
	}
	if id == "" {
		err = tx.QueryRowContext(ctx, `INSERT INTO parties(party_type,member_id,name,phone,email,address,active) VALUES($1,NULLIF($2,'')::uuid,$3,$4,NULLIF($5,''),$6,TRUE) RETURNING to_jsonb(parties)`, v.PartyType, v.MemberID, v.Name, v.Phone, v.Email, v.Address).Scan(&after)
	} else {
		err = tx.QueryRowContext(ctx, `UPDATE parties SET member_id=NULLIF($2,'')::uuid,name=$3,phone=$4,email=NULLIF($5,''),address=$6,active=$7,updated_at=now() WHERE id=$1 RETURNING to_jsonb(parties)`, id, v.MemberID, v.Name, v.Phone, v.Email, v.Address, v.Active).Scan(&after)
	}
	if err != nil {
		return nil, err
	}
	var snapshot map[string]any
	if err = json.Unmarshal(after, &snapshot); err != nil {
		return nil, err
	}
	entityID, _ := snapshot["id"].(string)
	action := "created"
	if id != "" {
		action = "updated"
	}
	if err = writeAudit(ctx, tx, actor, action, "parties", entityID, before, after); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, "ledger.party_saved.v1", "parties", entityID, after); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(after), nil
}

func (s *Service) DeactivateParty(ctx context.Context, actor Actor, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before, after []byte
	if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(p) FROM parties p WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `UPDATE parties SET active=FALSE,updated_at=now() WHERE id=$1 RETURNING to_jsonb(parties)`, id).Scan(&after); err != nil {
		return err
	}
	if err = writeAudit(ctx, tx, actor, "deactivated", "parties", id, before, after); err != nil {
		return err
	}
	if err = writeEvent(ctx, tx, "ledger.party_deactivated.v1", "parties", id, after); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) ListFinancialEntries(ctx context.Context, direction string) ([]json.RawMessage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(r) FROM (SELECT f.*,p.name AS party_name,p.party_type FROM financial_entries f JOIN parties p ON p.id=f.party_id WHERE f.direction=$1 ORDER BY f.occurred_at DESC LIMIT 200) r`, direction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) RecordFinancialEntry(ctx context.Context, actor Actor, direction string, v FinancialEntryInput) (json.RawMessage, error) {
	if direction != "receipt" && direction != "payment" {
		return nil, invalid("unknown financial entry direction")
	}
	if v.PartyID == "" || v.AmountMinor <= 0 || strings.TrimSpace(v.IdempotencyKey) == "" || len(v.IdempotencyKey) > 128 {
		return nil, invalid("partyId, idempotencyKey, and positive amountMinor are required")
	}
	if v.Currency == "" {
		v.Currency = "INR"
	}
	if !validCurrency(v.Currency) {
		return nil, invalid("currency must be a three-letter uppercase code")
	}
	if v.PaymentMethod == "" {
		v.PaymentMethod = "cash"
	}
	if v.PaymentMethod != "cash" && v.PaymentMethod != "upi" && v.PaymentMethod != "bank_transfer" && v.PaymentMethod != "card" && v.PaymentMethod != "cheque" && v.PaymentMethod != "other" {
		return nil, invalid("paymentMethod must be cash, upi, bank_transfer, card, cheque, or other")
	}
	var occurred any
	if v.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, v.OccurredAt)
		if err != nil {
			return nil, invalid("occurredAt must use RFC3339 format")
		}
		occurred = parsed
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var result []byte
	err = tx.QueryRowContext(ctx, `INSERT INTO financial_entries(direction,party_id,amount_minor,currency,payment_method,occurred_at,reference,description,idempotency_key,recorded_by) SELECT $1,p.id,$3,$4,$5,COALESCE($6::timestamptz,now()),$7,$8,$9,NULLIF($10,'')::uuid FROM parties p WHERE p.id=$2 AND p.active AND p.party_type=CASE WHEN $1='receipt' THEN 'customer' ELSE 'supplier' END ON CONFLICT(idempotency_key) DO NOTHING RETURNING to_jsonb(financial_entries)`, direction, v.PartyID, v.AmountMinor, v.Currency, v.PaymentMethod, occurred, v.Reference, v.Description, v.IdempotencyKey, actor.UserID).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		var existing []byte
		var oldDirection, oldParty, oldCurrency, oldMethod, oldReference, oldDescription string
		var amount int64
		lookupErr := s.DB.QueryRowContext(ctx, `SELECT to_jsonb(f),f.direction,f.party_id::text,f.amount_minor,f.currency,f.payment_method,f.reference,f.description FROM financial_entries f WHERE f.idempotency_key=$1`, v.IdempotencyKey).Scan(&existing, &oldDirection, &oldParty, &amount, &oldCurrency, &oldMethod, &oldReference, &oldDescription)
		if lookupErr == nil {
			if oldDirection == direction && oldParty == v.PartyID && amount == v.AmountMinor && oldCurrency == v.Currency && oldMethod == v.PaymentMethod && oldReference == v.Reference && oldDescription == v.Description {
				return camelJSON(existing), nil
			}
			return nil, invalid("idempotencyKey was already used for a different entry")
		}
		return nil, invalid("an active party of the correct type is required")
	}
	if err != nil {
		return nil, err
	}
	var snapshot map[string]any
	if err = json.Unmarshal(result, &snapshot); err != nil {
		return nil, err
	}
	id, _ := snapshot["id"].(string)
	action := "customer_receipt_recorded"
	eventType := "ledger.customer_receipt_recorded.v1"
	if direction == "payment" {
		action = "supplier_payment_recorded"
		eventType = "ledger.supplier_payment_recorded.v1"
	}
	if err = writeAudit(ctx, tx, actor, action, "financial_entries", id, nil, result); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, eventType, "financial_entries", id, result); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(result), nil
}

func (s *Service) FinancialReport(ctx context.Context, kind, from, to, partyID string) ([]json.RawMessage, error) {
	var query string
	switch kind {
	case "cashbook":
		query = `SELECT to_jsonb(r) FROM (SELECT occurred_at::date AS date,sum(amount_minor) FILTER(WHERE direction='receipt') AS receipts_minor,sum(amount_minor) FILTER(WHERE direction='payment') AS payments_minor,currency FROM financial_entries WHERE occurred_at >= $1::date AND occurred_at < ($2::date+interval '1 day') GROUP BY occurred_at::date,currency ORDER BY date) r`
	case "customer-receipts":
		query = `SELECT to_jsonb(r) FROM (SELECT f.id,f.entry_number,f.occurred_at,f.amount_minor,f.currency,f.payment_method,f.reference,f.description,p.id AS party_id,p.name AS party_name FROM financial_entries f JOIN parties p ON p.id=f.party_id WHERE f.direction='receipt' AND f.occurred_at >= $1::date AND f.occurred_at < ($2::date+interval '1 day') ORDER BY f.occurred_at DESC) r`
	case "supplier-payments":
		query = `SELECT to_jsonb(r) FROM (SELECT f.id,f.entry_number,f.occurred_at,f.amount_minor,f.currency,f.payment_method,f.reference,f.description,p.id AS party_id,p.name AS party_name FROM financial_entries f JOIN parties p ON p.id=f.party_id WHERE f.direction='payment' AND f.occurred_at >= $1::date AND f.occurred_at < ($2::date+interval '1 day') ORDER BY f.occurred_at DESC) r`
	case "party-ledger":
		if partyID == "" {
			return nil, invalid("partyId is required for the party ledger")
		}
		query = `SELECT to_jsonb(r) FROM (SELECT f.id,f.entry_number,f.direction,f.occurred_at,f.amount_minor,f.currency,f.payment_method,f.reference,f.description,p.name AS party_name,p.party_type FROM financial_entries f JOIN parties p ON p.id=f.party_id WHERE f.party_id=$3 AND f.occurred_at >= $1::date AND f.occurred_at < ($2::date+interval '1 day') ORDER BY f.occurred_at DESC) r`
	default:
		return nil, invalid("unknown financial report")
	}
	args := []any{from, to}
	if kind == "party-ledger" {
		args = append(args, partyID)
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) FinancialReceipt(ctx context.Context, id string) (json.RawMessage, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT to_jsonb(r) FROM (SELECT f.id,f.entry_number,f.direction,f.occurred_at,f.amount_minor,f.currency,f.payment_method,f.reference,f.description,p.name AS party_name,p.party_type,p.phone AS party_phone FROM financial_entries f JOIN parties p ON p.id=f.party_id WHERE f.id=$1) r`, id).Scan(&raw)
	return camelJSON(raw), err
}

// SaveMaster performs the master-data mutation and its audit record atomically.
// These are complete representations (PUT semantics), so omitted fields take defaults.
func (s *Service) SaveMaster(ctx context.Context, actor Actor, kind, id string, input json.RawMessage) (json.RawMessage, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var before []byte
	var table, eventType string
	var after []byte
	switch kind {
	case "promoters":
		var v Promoter
		if err = json.Unmarshal(input, &v); err != nil {
			return nil, invalid("invalid promoter data")
		}
		if strings.TrimSpace(v.Name) == "" || strings.TrimSpace(v.Phone) == "" {
			return nil, invalid("name and phone are required")
		}
		if v.UserID != "" {
			var role string
			if err = tx.QueryRowContext(ctx, `SELECT role FROM app_users WHERE id=$1`, v.UserID).Scan(&role); err != nil {
				return nil, err
			}
			if role != "promoter" {
				return nil, invalid("linked account must have promoter role")
			}
		}
		table, eventType = "promoters", "master.promoter_saved.v1"
		if id == "" {
			err = tx.QueryRowContext(ctx, `INSERT INTO promoters(user_id,name,phone,email,address,active) VALUES(NULLIF($1,'')::uuid,$2,$3,NULLIF($4,''),$5,TRUE) RETURNING to_jsonb(promoters)`, v.UserID, v.Name, v.Phone, v.Email, v.Address).Scan(&after)
		} else {
			if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(p) FROM promoters p WHERE id=$1 FOR UPDATE`, id).Scan(&before); err == nil {
				err = tx.QueryRowContext(ctx, `UPDATE promoters SET user_id=NULLIF($2,'')::uuid,name=$3,phone=$4,email=NULLIF($5,''),address=$6,active=$7,updated_at=now() WHERE id=$1 RETURNING to_jsonb(promoters)`, id, v.UserID, v.Name, v.Phone, v.Email, v.Address, v.Active).Scan(&after)
			}
		}
	case "members":
		var v Member
		if err = json.Unmarshal(input, &v); err != nil {
			return nil, invalid("invalid member data")
		}
		if strings.TrimSpace(v.Name) == "" || strings.TrimSpace(v.Phone) == "" || v.PromoterID == "" {
			return nil, invalid("promoterId, name, and phone are required")
		}
		if v.UserID != "" {
			var role string
			if err = tx.QueryRowContext(ctx, `SELECT role FROM app_users WHERE id=$1`, v.UserID).Scan(&role); err != nil {
				return nil, err
			}
			if role != "member" {
				return nil, invalid("linked account must have member role")
			}
		}
		if v.KYCLast4 != "" && (len(v.KYCLast4) != 4 || strings.Trim(v.KYCLast4, "0123456789") != "") {
			return nil, invalid("kycLast4 must be four digits")
		}
		if v.KYCType != "" && v.KYCType != "aadhaar" && v.KYCType != "pan" && v.KYCType != "voter_id" {
			return nil, invalid("kycType must be aadhaar, pan, or voter_id")
		}
		table, eventType = "members", "master.member_saved.v1"
		if id == "" {
			err = tx.QueryRowContext(ctx, `INSERT INTO members(user_id,promoter_id,name,phone,email,address,kyc_type,kyc_last4,kyc_verified_at,active) VALUES(NULLIF($1,'')::uuid,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),NULLIF($8,''),$9,TRUE) RETURNING to_jsonb(members)`, v.UserID, v.PromoterID, v.Name, v.Phone, v.Email, v.Address, v.KYCType, v.KYCLast4, v.KYCVerifiedAt).Scan(&after)
		} else {
			if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(m) FROM members m WHERE id=$1 FOR UPDATE`, id).Scan(&before); err == nil {
				err = tx.QueryRowContext(ctx, `UPDATE members SET user_id=NULLIF($2,'')::uuid,promoter_id=$3,name=$4,phone=$5,email=NULLIF($6,''),address=$7,kyc_type=NULLIF($8,''),kyc_last4=NULLIF($9,''),kyc_verified_at=$10,active=$11,updated_at=now() WHERE id=$1 RETURNING to_jsonb(members)`, id, v.UserID, v.PromoterID, v.Name, v.Phone, v.Email, v.Address, v.KYCType, v.KYCLast4, v.KYCVerifiedAt, v.Active).Scan(&after)
			}
		}
	case "schemes":
		var v Scheme
		if err = json.Unmarshal(input, &v); err != nil {
			return nil, invalid("invalid scheme data")
		}
		if strings.TrimSpace(v.Name) == "" || v.ContributionMinor <= 0 || v.DurationMonths <= 0 {
			return nil, invalid("name, positive contributionMinor, and positive durationMonths are required")
		}
		if v.Currency == "" {
			v.Currency = "INR"
		}
		if !validCurrency(v.Currency) {
			return nil, invalid("currency must be a three-letter uppercase code")
		}
		table, eventType = "schemes", "master.scheme_saved.v1"
		if id == "" {
			err = tx.QueryRowContext(ctx, `INSERT INTO schemes(name,description,contribution_minor,duration_months,currency,active) VALUES($1,$2,$3,$4,$5,TRUE) RETURNING to_jsonb(schemes)`, v.Name, v.Description, v.ContributionMinor, v.DurationMonths, v.Currency).Scan(&after)
		} else {
			if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(s) FROM schemes s WHERE id=$1 FOR UPDATE`, id).Scan(&before); err == nil {
				err = tx.QueryRowContext(ctx, `UPDATE schemes SET name=$2,description=$3,contribution_minor=$4,duration_months=$5,currency=$6,active=$7,updated_at=now() WHERE id=$1 RETURNING to_jsonb(schemes)`, id, v.Name, v.Description, v.ContributionMinor, v.DurationMonths, v.Currency, v.Active).Scan(&after)
			}
		}
	default:
		return nil, invalid("unknown resource")
	}
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err = json.Unmarshal(after, &obj); err != nil {
		return nil, err
	}
	entityID, _ := obj["id"].(string)
	if err = writeAudit(ctx, tx, actor, func() string {
		if id == "" {
			return "created"
		}
		return "updated"
	}(), table, entityID, before, after); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, eventType, table, entityID, after); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(after), nil
}

func (s *Service) DeactivateMaster(ctx context.Context, actor Actor, kind, id string) error {
	tables := map[string]string{"promoters": "promoters", "members": "members", "schemes": "schemes"}
	table, ok := tables[kind]
	if !ok {
		return invalid("unknown resource")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before, after []byte
	if err = tx.QueryRowContext(ctx, "SELECT to_jsonb(x) FROM "+table+" x WHERE id=$1 FOR UPDATE", id).Scan(&before); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, "UPDATE "+table+" SET active=FALSE,updated_at=now() WHERE id=$1 RETURNING to_jsonb("+table+")", id).Scan(&after); err != nil {
		return err
	}
	var obj map[string]any
	if err = json.Unmarshal(after, &obj); err != nil {
		return err
	}
	entityID, _ := obj["id"].(string)
	if err = writeAudit(ctx, tx, actor, "deactivated", table, entityID, before, after); err != nil {
		return err
	}
	if err = writeEvent(ctx, tx, "master."+strings.TrimSuffix(table, "s")+"_deactivated.v1", table, entityID, after); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) RecordPayment(ctx context.Context, actor Actor, v PaymentInput) (json.RawMessage, error) {
	if v.AmountMinor <= 0 || v.MemberID == "" || v.EnrollmentID == "" || strings.TrimSpace(v.IdempotencyKey) == "" || len(v.IdempotencyKey) > 128 {
		return nil, invalid("idempotencyKey, memberId, enrollmentId, and positive amountMinor are required")
	}
	if v.Currency == "" {
		v.Currency = "INR"
	}
	if v.PaymentMethod == "" {
		v.PaymentMethod = "cash"
	}
	if v.PaymentMethod != "cash" && v.PaymentMethod != "upi" && v.PaymentMethod != "bank_transfer" && v.PaymentMethod != "card" && v.PaymentMethod != "other" {
		return nil, invalid("paymentMethod must be cash, upi, bank_transfer, card, or other")
	}
	if !validCurrency(v.Currency) {
		return nil, invalid("currency must be a three-letter uppercase code")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var result []byte
	err = tx.QueryRowContext(ctx, `INSERT INTO payments(member_id,enrollment_id,amount_minor,currency,payment_method,reference,idempotency_key,received_by) SELECT $1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid WHERE EXISTS (SELECT 1 FROM enrollments WHERE id=$2 AND member_id=$1 AND status='active') AND EXISTS (SELECT 1 FROM members WHERE id=$1 AND active) AND ($9='admin' OR EXISTS (SELECT 1 FROM members m JOIN promoters p ON p.id=m.promoter_id WHERE m.id=$1 AND p.user_id=NULLIF($8,'')::uuid AND p.active)) ON CONFLICT (idempotency_key) DO NOTHING RETURNING to_jsonb(payments)`, v.MemberID, v.EnrollmentID, v.AmountMinor, v.Currency, v.PaymentMethod, v.Reference, v.IdempotencyKey, actor.UserID, actor.Role).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		var existing []byte
		var memberID, enrollmentID, currency, method, reference string
		var amount int64
		lookupErr := s.DB.QueryRowContext(ctx, `SELECT to_jsonb(p),p.member_id::text,p.enrollment_id::text,p.amount_minor,p.currency,p.payment_method,p.reference FROM payments p WHERE p.idempotency_key=$1 AND ($2='admin' OR EXISTS (SELECT 1 FROM members m JOIN promoters pr ON pr.id=m.promoter_id WHERE m.id=p.member_id AND ((p.received_by=$3::uuid) OR (pr.user_id=$3::uuid AND pr.active))))`, v.IdempotencyKey, actor.Role, actor.UserID).Scan(&existing, &memberID, &enrollmentID, &amount, &currency, &method, &reference)
		if lookupErr == nil {
			if memberID == v.MemberID && enrollmentID == v.EnrollmentID && amount == v.AmountMinor && currency == v.Currency && method == v.PaymentMethod && reference == v.Reference {
				return camelJSON(existing), nil
			}
			return nil, invalid("idempotencyKey was already used for a different payment")
		}
		return nil, invalid("active enrollment not found for member or account")
	}
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err = json.Unmarshal(result, &obj); err != nil {
		return nil, err
	}
	id, _ := obj["id"].(string)
	if err = writeAudit(ctx, tx, actor, "payment_recorded", "payments", id, nil, result); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, "scheme.payment_recorded.v1", "payments", id, result); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(result), nil
}

func (s *Service) CreateEnrollment(ctx context.Context, actor Actor, memberID, schemeID, startMonth string) (json.RawMessage, error) {
	month, err := time.Parse("2006-01", startMonth)
	if err != nil {
		return nil, invalid("startMonth must use YYYY-MM")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var result []byte
	err = tx.QueryRowContext(ctx, `INSERT INTO enrollments(member_id,scheme_id,start_month) SELECT m.id,s.id,$3 FROM members m JOIN schemes s ON TRUE WHERE m.id=$1 AND s.id=$2 AND m.active AND s.active RETURNING to_jsonb(enrollments)`, memberID, schemeID, month).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, invalid("active member and scheme are required")
		}
		return nil, err
	}
	var obj map[string]any
	if err = json.Unmarshal(result, &obj); err != nil {
		return nil, err
	}
	id, _ := obj["id"].(string)
	if err = writeAudit(ctx, tx, actor, "enrollment_created", "enrollments", id, nil, result); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, "scheme.enrollment_created.v1", "enrollments", id, result); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(result), nil
}

func (s *Service) UpdateEnrollmentStatus(ctx context.Context, actor Actor, id, status string) (json.RawMessage, error) {
	if status != "active" && status != "paused" && status != "completed" && status != "cancelled" {
		return nil, invalid("status must be active, paused, completed, or cancelled")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var before, after []byte
	if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(e) FROM enrollments e WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `UPDATE enrollments SET status=$2 WHERE id=$1 RETURNING to_jsonb(enrollments)`, id, status).Scan(&after); err != nil {
		return nil, err
	}
	if err = writeAudit(ctx, tx, actor, "enrollment_"+status, "enrollments", id, before, after); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, "scheme.enrollment_"+status+".v1", "enrollments", id, after); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(after), nil
}

func (s *Service) CreateIncentive(ctx context.Context, actor Actor, promoterID, period string, amount, commission int64, currency, description string) (json.RawMessage, error) {
	month, err := time.Parse("2006-01", period)
	if err != nil {
		return nil, invalid("period must use YYYY-MM")
	}
	if amount < 0 || commission < 0 {
		return nil, invalid("amountMinor and commissionMinor cannot be negative")
	}
	if currency == "" {
		currency = "INR"
	}
	if !validCurrency(currency) {
		return nil, invalid("currency must be a three-letter uppercase code")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var result []byte
	err = tx.QueryRowContext(ctx, `INSERT INTO promoter_incentives(promoter_id,period,amount_minor,commission_minor,currency,description,created_by) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid) RETURNING to_jsonb(promoter_incentives)`, promoterID, month, amount, commission, currency, description, actor.UserID).Scan(&result)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err = json.Unmarshal(result, &obj); err != nil {
		return nil, err
	}
	id, _ := obj["id"].(string)
	if err = writeAudit(ctx, tx, actor, "incentive_created", "promoter_incentives", id, nil, result); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, "scheme.promoter_incentive_created.v1", "promoter_incentives", id, result); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(result), nil
}

func (s *Service) UpdateIncentiveStatus(ctx context.Context, actor Actor, id, status string) (json.RawMessage, error) {
	if status != "approved" && status != "paid" {
		return nil, invalid("status must be approved or paid")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var before, after []byte
	if err = tx.QueryRowContext(ctx, `SELECT to_jsonb(i) FROM promoter_incentives i WHERE id=$1 FOR UPDATE`, id).Scan(&before); err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `UPDATE promoter_incentives SET status=$2 WHERE id=$1 AND (($2='approved' AND status='draft') OR ($2='paid' AND status='approved')) RETURNING to_jsonb(promoter_incentives)`, id, status).Scan(&after); err != nil {
		return nil, err
	}
	if err = writeAudit(ctx, tx, actor, "incentive_"+status, "promoter_incentives", id, before, after); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, "scheme.promoter_incentive_"+status+".v1", "promoter_incentives", id, after); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(after), nil
}

func (s *Service) Receipt(ctx context.Context, actor Actor, id string) (json.RawMessage, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT to_jsonb(r) FROM (SELECT p.id,p.receipt_number,p.paid_at,p.amount_minor,p.currency,p.payment_method,p.reference,m.member_number,m.name AS member_name,m.phone AS member_phone,pr.name AS promoter_name FROM payments p JOIN members m ON m.id=p.member_id JOIN promoters pr ON pr.id=m.promoter_id WHERE p.id=$1 AND ($2 IN ('admin','staff') OR pr.user_id=NULLIF($3,'')::uuid OR m.user_id=NULLIF($3,'')::uuid)) r`, id, actor.Role, actor.UserID).Scan(&raw)
	return camelJSON(raw), err
}

func (s *Service) MyMembers(ctx context.Context, userID string) ([]json.RawMessage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(m)-'kyc_last4'-'user_id' FROM members m JOIN promoters p ON p.id=m.promoter_id WHERE p.user_id=$1 ORDER BY m.member_number LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) MyMember(ctx context.Context, userID string) (json.RawMessage, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT to_jsonb(m)-'kyc_last4'-'user_id' FROM members m WHERE m.user_id=$1 AND m.active`, userID).Scan(&raw)
	return camelJSON(raw), err
}

func (s *Service) MyPayments(ctx context.Context, userID string) ([]json.RawMessage, error) {
	return s.PaymentsFor(ctx, "member", userID)
}

func (s *Service) PaymentsFor(ctx context.Context, role, userID string) ([]json.RawMessage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(r) FROM (SELECT p.id,p.receipt_number,p.paid_at,p.amount_minor,p.currency,p.payment_method,s.name AS scheme_name,m.name AS member_name FROM payments p JOIN members m ON m.id=p.member_id JOIN promoters pr ON pr.id=m.promoter_id JOIN enrollments e ON e.id=p.enrollment_id JOIN schemes s ON s.id=e.scheme_id WHERE ($1 IN ('admin','staff') OR ($1='promoter' AND pr.user_id=$2) OR ($1='member' AND m.user_id=$2)) ORDER BY p.paid_at DESC LIMIT 200) r`, role, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) EnrollmentsFor(ctx context.Context, role, userID string) ([]json.RawMessage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(r) FROM (SELECT e.id,e.member_id,e.scheme_id,e.start_month,e.status,m.member_number,m.name AS member_name,pr.name AS promoter_name,s.name AS scheme_name,s.contribution_minor,s.currency,s.duration_months FROM enrollments e JOIN members m ON m.id=e.member_id JOIN promoters pr ON pr.id=m.promoter_id JOIN schemes s ON s.id=e.scheme_id WHERE ($1 IN ('admin','staff') OR ($1='promoter' AND pr.user_id=$2) OR ($1='member' AND m.user_id=$2)) ORDER BY e.start_month DESC LIMIT 200) r`, role, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) PublicSchemes(ctx context.Context) ([]json.RawMessage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(s) FROM schemes s WHERE active ORDER BY name LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) MyIncentives(ctx context.Context, userID string) ([]json.RawMessage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(r) FROM (SELECT i.id,i.period,i.amount_minor,i.commission_minor,(i.amount_minor+i.commission_minor) AS total_minor,i.currency,i.description,i.status FROM promoter_incentives i JOIN promoters p ON p.id=i.promoter_id WHERE p.user_id=$1 ORDER BY i.period DESC LIMIT 120) r`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) Audit(ctx context.Context, limit int) ([]json.RawMessage, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT to_jsonb(a) FROM audit_entries a ORDER BY occurred_at DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}

func (s *Service) RecordDraw(ctx context.Context, actor Actor, v DrawInput) (json.RawMessage, error) {
	month, err := time.Parse("2006-01", v.DrawMonth)
	if err != nil {
		return nil, invalid("drawMonth must use YYYY-MM")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var result []byte
	err = tx.QueryRowContext(ctx, `INSERT INTO draw_results(scheme_id,draw_month,member_id,recorded_by,notes) SELECT $1,$2,$3,NULLIF($4,'')::uuid,$5 WHERE EXISTS (SELECT 1 FROM enrollments e JOIN members m ON m.id=e.member_id JOIN schemes s ON s.id=e.scheme_id WHERE e.scheme_id=$1 AND e.member_id=$3 AND e.status='active' AND m.active AND s.active) RETURNING to_jsonb(draw_results)`, v.SchemeID, month, v.MemberID, actor.UserID, v.Notes).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("chosen member is not actively enrolled in this scheme")
	}
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err = json.Unmarshal(result, &obj); err != nil {
		return nil, err
	}
	id, _ := obj["id"].(string)
	if err = writeAudit(ctx, tx, actor, "draw_result_recorded", "draw_results", id, nil, result); err != nil {
		return nil, err
	}
	if err = writeEvent(ctx, tx, "scheme.draw_result_recorded.v1", "draw_results", id, result); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return camelJSON(result), nil
}

func writeAudit(ctx context.Context, tx *sql.Tx, actor Actor, action, entity, id string, before, after []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_entries(actor_user_id,action,entity_type,entity_id,request_id,before_state,after_state) VALUES(NULLIF($1,'')::uuid,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')::jsonb,NULLIF($7,'')::jsonb)`, actor.UserID, action, entity, id, actor.RequestID, string(camelJSON(before)), string(camelJSON(after)))
	return err
}
func writeEvent(ctx context.Context, tx *sql.Tx, eventType, aggregateType, aggregateID string, payload []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO event_outbox(event_type,aggregate_type,aggregate_id,payload) VALUES($1,$2,$3,$4::jsonb)`, eventType, aggregateType, aggregateID, string(camelJSON(payload)))
	return err
}

func (s *Service) Report(ctx context.Context, kind, from, to string) ([]json.RawMessage, error) {
	var query string
	switch kind {
	case "payments":
		query = `SELECT to_jsonb(r) FROM (SELECT p.id,p.receipt_number,p.paid_at,p.amount_minor,p.currency,p.payment_method,p.reference,m.member_number,m.name AS member_name,pr.name AS promoter_name FROM payments p JOIN members m ON m.id=p.member_id JOIN promoters pr ON pr.id=m.promoter_id WHERE p.paid_at >= $1::date AND p.paid_at < ($2::date + interval '1 day') ORDER BY p.paid_at DESC) r`
	case "daybook":
		query = `SELECT to_jsonb(r) FROM (SELECT p.paid_at::date AS date,count(*) AS receipt_count,sum(p.amount_minor) AS total_minor,p.currency FROM payments p WHERE p.paid_at >= $1::date AND p.paid_at < ($2::date + interval '1 day') GROUP BY p.paid_at::date,p.currency ORDER BY date) r`
	case "dues":
		query = `SELECT to_jsonb(r) FROM (SELECT pr.id AS promoter_id,pr.name AS promoter_name,m.id AS member_id,m.member_number,m.name AS member_name,s.name AS scheme_name,e.start_month,s.contribution_minor AS monthly_contribution_minor,COALESCE(paid.paid_minor,0) AS paid_minor,GREATEST(0,LEAST(s.duration_months,((EXTRACT(YEAR FROM age(date_trunc('month',$2::date),e.start_month))::int*12)+EXTRACT(MONTH FROM age(date_trunc('month',$2::date),e.start_month))::int+1))*s.contribution_minor-COALESCE(paid.paid_minor,0)) AS total_due_minor FROM enrollments e JOIN members m ON m.id=e.member_id JOIN promoters pr ON pr.id=m.promoter_id JOIN schemes s ON s.id=e.scheme_id LEFT JOIN LATERAL (SELECT sum(amount_minor) AS paid_minor FROM payments WHERE enrollment_id=e.id AND paid_at < ($2::date + interval '1 day')) paid ON TRUE WHERE e.status='active' AND e.start_month <= date_trunc('month',$2::date) AND GREATEST(0,LEAST(s.duration_months,((EXTRACT(YEAR FROM age(date_trunc('month',$2::date),e.start_month))::int*12)+EXTRACT(MONTH FROM age(date_trunc('month',$2::date),e.start_month))::int+1))*s.contribution_minor-COALESCE(paid.paid_minor,0))>0 ORDER BY pr.name,m.name) r`
	case "incentives":
		query = `SELECT to_jsonb(r) FROM (SELECT i.*,pr.name AS promoter_name,(i.amount_minor+i.commission_minor) AS total_minor FROM promoter_incentives i JOIN promoters pr ON pr.id=i.promoter_id WHERE i.period >= $1::date AND i.period < ($2::date + interval '1 day') ORDER BY i.period DESC,pr.name) r`
	case "draw-results":
		query = `SELECT to_jsonb(r) FROM (SELECT d.id,d.draw_month,d.chosen_at,d.notes,s.name AS scheme_name,m.member_number,m.name AS member_name FROM draw_results d JOIN schemes s ON s.id=d.scheme_id JOIN members m ON m.id=d.member_id WHERE d.draw_month >= $1::date AND d.draw_month < ($2::date + interval '1 day') ORDER BY d.draw_month DESC,s.name) r`
	default:
		return nil, invalid("unknown report")
	}
	query += ` LIMIT 5000`
	rows, err := s.DB.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		items = append(items, camelJSON(raw))
	}
	return items, rows.Err()
}
