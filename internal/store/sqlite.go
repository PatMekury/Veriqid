package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store wraps a SQLite database for user and challenge persistence.
type Store struct {
	db *sql.DB
}

// User represents a registered platform user.
type User struct {
	SpkHex     string
	Username   string
	AgeBracket int
	CreatedAt  time.Time
	LastLogin  time.Time
}

// NewStore opens (or creates) the SQLite database at the given path
// and initializes the schema.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		spk_hex     TEXT PRIMARY KEY,
		username    TEXT NOT NULL,
		age_bracket INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login  DATETIME
	);

	CREATE TABLE IF NOT EXISTS challenges (
		challenge_hex  TEXT PRIMARY KEY,
		challenge_type TEXT NOT NULL,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
		used           BOOLEAN DEFAULT FALSE
	);

	CREATE INDEX IF NOT EXISTS idx_challenges_created ON challenges(created_at);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Challenge Management ---

// StoreChallenge saves a newly generated challenge for later verification.
func (s *Store) StoreChallenge(challengeHex, challengeType string) error {
	_, err := s.db.Exec(
		"INSERT INTO challenges (challenge_hex, challenge_type) VALUES (?, ?)",
		challengeHex, challengeType,
	)
	return err
}

// ValidateChallenge checks if a challenge exists, is unused, and is not expired.
// If valid, marks it as used (one-time use).
func (s *Store) ValidateChallenge(challengeHex, expectedType string) (bool, error) {
	expiry := time.Now().Add(-5 * time.Minute)

	result, err := s.db.Exec(
		`UPDATE challenges
		 SET used = TRUE
		 WHERE challenge_hex = ?
		   AND challenge_type = ?
		   AND used = FALSE
		   AND created_at > ?`,
		challengeHex, expectedType, expiry,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

// CleanExpiredChallenges removes challenges older than 10 minutes.
func (s *Store) CleanExpiredChallenges() error {
	expiry := time.Now().Add(-10 * time.Minute)
	_, err := s.db.Exec("DELETE FROM challenges WHERE created_at < ?", expiry)
	return err
}

// --- User Management ---

// RegisterUser stores a new user after successful signup.
func (s *Store) RegisterUser(spkHex, username string, ageBracket int) error {
	_, err := s.db.Exec(
		`INSERT INTO users (spk_hex, username, age_bracket, last_login)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		spkHex, username, ageBracket,
	)
	if err != nil {
		return fmt.Errorf("failed to register user: %w (spk may already exist)", err)
	}
	return nil
}

// GetUserBySPK retrieves a user by their service-specific public key.
func (s *Store) GetUserBySPK(spkHex string) (*User, error) {
	row := s.db.QueryRow(
		"SELECT spk_hex, username, age_bracket, created_at, last_login FROM users WHERE spk_hex = ?",
		spkHex,
	)

	var u User
	var lastLogin sql.NullTime
	err := row.Scan(&u.SpkHex, &u.Username, &u.AgeBracket, &u.CreatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLogin = lastLogin.Time
	}
	return &u, nil
}

// UpdateLastLogin updates the last_login timestamp for a user.
func (s *Store) UpdateLastLogin(spkHex string) error {
	_, err := s.db.Exec(
		"UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE spk_hex = ?",
		spkHex,
	)
	return err
}

// UserExists checks if a user with the given SPK is already registered.
func (s *Store) UserExists(spkHex string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE spk_hex = ?", spkHex).Scan(&count)
	return count > 0, err
}

// GetUserCount returns the total number of registered users.
func (s *Store) GetUserCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetAllUsers returns all registered users.
func (s *Store) GetAllUsers() ([]User, error) {
	rows, err := s.db.Query(
		"SELECT spk_hex, username, age_bracket, created_at, last_login FROM users ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.SpkHex, &u.Username, &u.AgeBracket, &u.CreatedAt, &lastLogin); err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			u.LastLogin = lastLogin.Time
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
