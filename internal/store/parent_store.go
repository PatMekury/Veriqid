package store

import (
	"database/sql"
	"fmt"
)

// Parent represents a parent account row.
type Parent struct {
	ID            int64
	Email         *string // nullable (phone-based accounts have no email)
	Phone         *string // nullable (email-based accounts have no phone)
	PasswordHash  *string // nullable (phone-based accounts have no password)
	EthAddress    string
	EthPrivkeyEnc string
}

// Child represents a child row.
type Child struct {
	ID            int64
	ParentID      int64
	DisplayName   string
	AgeBracket    int
	MpkHex        *string
	ContractIndex *int
	Status        string
	VerifiedAt    *string
	RevokedAt     *string
	CreatedAt     string
}

// ── Parent Methods ──────────────────────────────────────────

// CreateParentByEmail creates a parent account using email + bcrypt password hash.
func (s *Store) CreateParentByEmail(email, passwordHash, ethAddress, ethPrivkeyEnc string) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO parents (email, password_hash, eth_address, eth_privkey_enc) VALUES (?, ?, ?, ?)`,
		email, passwordHash, ethAddress, ethPrivkeyEnc,
	)
	if err != nil {
		return 0, fmt.Errorf("create parent: %w", err)
	}
	return result.LastInsertId()
}

// CreateParentByPhone creates a parent account using phone number (OTP-based login).
func (s *Store) CreateParentByPhone(phone, ethAddress, ethPrivkeyEnc string) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO parents (phone, eth_address, eth_privkey_enc) VALUES (?, ?, ?)`,
		phone, ethAddress, ethPrivkeyEnc,
	)
	if err != nil {
		return 0, fmt.Errorf("create parent: %w", err)
	}
	return result.LastInsertId()
}

// GetParentByEmail retrieves a parent by email.
func (s *Store) GetParentByEmail(email string) (*Parent, error) {
	row := s.db.QueryRow(
		`SELECT id, email, phone, password_hash, eth_address, eth_privkey_enc FROM parents WHERE email = ?`,
		email,
	)
	p := &Parent{}
	err := row.Scan(&p.ID, &p.Email, &p.Phone, &p.PasswordHash, &p.EthAddress, &p.EthPrivkeyEnc)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("parent not found")
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetParentByPhone retrieves a parent by phone number.
func (s *Store) GetParentByPhone(phone string) (*Parent, error) {
	row := s.db.QueryRow(
		`SELECT id, email, phone, password_hash, eth_address, eth_privkey_enc FROM parents WHERE phone = ?`,
		phone,
	)
	p := &Parent{}
	err := row.Scan(&p.ID, &p.Email, &p.Phone, &p.PasswordHash, &p.EthAddress, &p.EthPrivkeyEnc)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("parent not found")
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetParentByID retrieves a parent by their ID.
func (s *Store) GetParentByID(parentID int64) (*Parent, error) {
	row := s.db.QueryRow(
		`SELECT id, email, phone, password_hash, eth_address, eth_privkey_enc FROM parents WHERE id = ?`,
		parentID,
	)
	p := &Parent{}
	err := row.Scan(&p.ID, &p.Email, &p.Phone, &p.PasswordHash, &p.EthAddress, &p.EthPrivkeyEnc)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("parent not found")
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateParentLastLogin updates the parent's last login timestamp.
func (s *Store) UpdateParentLastLogin(parentID int64) error {
	_, err := s.db.Exec(
		`UPDATE parents SET last_login = CURRENT_TIMESTAMP WHERE id = ?`, parentID,
	)
	return err
}

// ── Child Methods ───────────────────────────────────────────

// AddChild adds a child record linked to a parent.
func (s *Store) AddChild(parentID int64, displayName string, ageBracket int, mskEnc, mpkHex string) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO children (parent_id, display_name, age_bracket, msk_enc, mpk_hex, status)
		 VALUES (?, ?, ?, ?, ?, 'pending')`,
		parentID, displayName, ageBracket, mskEnc, mpkHex,
	)
	if err != nil {
		return 0, fmt.Errorf("add child: %w", err)
	}
	return result.LastInsertId()
}

// GetChildrenByParent returns all children for a given parent ID.
func (s *Store) GetChildrenByParent(parentID int64) ([]Child, error) {
	rows, err := s.db.Query(
		`SELECT id, parent_id, display_name, age_bracket, mpk_hex, contract_index, status, verified_at, revoked_at, created_at
		 FROM children WHERE parent_id = ? ORDER BY created_at DESC`,
		parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	defer rows.Close()

	var children []Child
	for rows.Next() {
		c := Child{}
		err := rows.Scan(&c.ID, &c.ParentID, &c.DisplayName, &c.AgeBracket, &c.MpkHex,
			&c.ContractIndex, &c.Status, &c.VerifiedAt, &c.RevokedAt, &c.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}
		children = append(children, c)
	}
	return children, nil
}

// GetChildByID retrieves a single child by ID.
func (s *Store) GetChildByID(childID int64) (*Child, error) {
	row := s.db.QueryRow(
		`SELECT id, parent_id, display_name, age_bracket, mpk_hex, contract_index, status, verified_at, revoked_at, created_at
		 FROM children WHERE id = ?`,
		childID,
	)
	c := &Child{}
	err := row.Scan(&c.ID, &c.ParentID, &c.DisplayName, &c.AgeBracket, &c.MpkHex,
		&c.ContractIndex, &c.Status, &c.VerifiedAt, &c.RevokedAt, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("child not found")
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// MarkChildVerified updates a child's status after successful verification.
func (s *Store) MarkChildVerified(childID int64, contractIndex int) error {
	_, err := s.db.Exec(
		`UPDATE children SET status = 'verified', contract_index = ?, verified_at = CURRENT_TIMESTAMP WHERE id = ?`,
		contractIndex, childID,
	)
	return err
}

// MarkChildRevoked updates a child's status after revocation.
func (s *Store) MarkChildRevoked(childID int64) error {
	_, err := s.db.Exec(
		`UPDATE children SET status = 'revoked', revoked_at = CURRENT_TIMESTAMP WHERE id = ?`,
		childID,
	)
	return err
}

// ── Platform Activity Methods ────────────────────────────────

// PlatformActivity represents a platform registration/activity event.
type PlatformActivity struct {
	ID            int64
	ContractIndex *int
	ServiceName   string
	SpkHex        string
	EventType     string
	EventTime     string
	CreatedAt     string
}

// AddPlatformActivity records that a child interacted with a third-party platform.
func (s *Store) AddPlatformActivity(contractIndex *int, serviceName, spkHex, eventType, eventTime string) error {
	_, err := s.db.Exec(
		`INSERT INTO platform_activity (contract_index, service_name, spk_hex, event_type, event_time) VALUES (?, ?, ?, ?, ?)`,
		contractIndex, serviceName, spkHex, eventType, eventTime,
	)
	return err
}

// GetPlatformActivityByContractIndex returns platform events for a given contract_index.
func (s *Store) GetPlatformActivityByContractIndex(contractIndex int) ([]PlatformActivity, error) {
	rows, err := s.db.Query(
		`SELECT id, contract_index, service_name, spk_hex, event_type, event_time, created_at
		 FROM platform_activity WHERE contract_index = ? ORDER BY event_time DESC`,
		contractIndex,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []PlatformActivity
	for rows.Next() {
		a := PlatformActivity{}
		err := rows.Scan(&a.ID, &a.ContractIndex, &a.ServiceName, &a.SpkHex, &a.EventType, &a.EventTime, &a.CreatedAt)
		if err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, nil
}

// GetAllPlatformActivityForParent returns all platform activity for all children of a parent.
func (s *Store) GetAllPlatformActivityForParent(parentID int64) ([]PlatformActivity, error) {
	rows, err := s.db.Query(
		`SELECT pa.id, pa.contract_index, pa.service_name, pa.spk_hex, pa.event_type, pa.event_time, pa.created_at
		 FROM platform_activity pa
		 INNER JOIN children c ON pa.contract_index = c.contract_index
		 WHERE c.parent_id = ?
		 ORDER BY pa.event_time DESC`,
		parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []PlatformActivity
	for rows.Next() {
		a := PlatformActivity{}
		err := rows.Scan(&a.ID, &a.ContractIndex, &a.ServiceName, &a.SpkHex, &a.EventType, &a.EventTime, &a.CreatedAt)
		if err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, nil
}

// ── OTP Methods ─────────────────────────────────────────────

// StoreOTP saves a 6-digit OTP for a phone number with 5-minute expiry.
func (s *Store) StoreOTP(phone, code string) error {
	_, err := s.db.Exec(
		`INSERT INTO otp_codes (phone, code, expires_at) VALUES (?, ?, datetime('now', '+5 minutes'))`,
		phone, code,
	)
	return err
}

// VerifyOTP checks if the OTP is valid, not expired, and not yet used. Marks it used if valid.
func (s *Store) VerifyOTP(phone, code string) (bool, error) {
	// Debug: dump all OTP rows for this phone
	rows, _ := s.db.Query(`SELECT id, phone, code, expires_at, used, datetime('now') as now_utc FROM otp_codes WHERE phone = ?`, phone)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var p, c, exp, nowUTC string
			var used bool
			rows.Scan(&id, &p, &c, &exp, &used, &nowUTC)
			fmt.Printf("[DEBUG OTP DB] id=%d phone=%q code=%q expires=%s used=%v now_utc=%s\n", id, p, c, exp, used, nowUTC)
		}
	}

	result, err := s.db.Exec(
		`UPDATE otp_codes SET used = 1
		 WHERE phone = ? AND code = ? AND used = 0 AND expires_at > datetime('now')`,
		phone, code,
	)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	fmt.Printf("[DEBUG OTP] rows affected: %d\n", affected)
	return affected > 0, nil
}

// CleanExpiredOTPs removes OTP codes older than 10 minutes.
func (s *Store) CleanExpiredOTPs() error {
	_, err := s.db.Exec("DELETE FROM otp_codes WHERE expires_at < datetime('now', '-10 minutes')")
	return err
}
