# Veriqid Phase 4: Veriqid Server — Complete Detailed Guide

**Time estimate: ~4 hours**
**Goal: Fork `cmd/server/main.go` into a production-ready Veriqid-branded service demo with configurable service names, session management, SQLite for SPK/user persistence, age-verification endpoints, and a platform SDK verification endpoint.**

---

> **IMPORTANT — Restarting Servers for Future Phases**
>
> If you shut down Ganache, the bridge, or the server at any point (between sessions, between phases, etc.), you will need to restart them before doing any work that touches the blockchain or bridge:
>
> 1. **Start Ganache** (Terminal 1):
>    ```bash
>    ganache --port 7545
>    ```
> 2. **Redeploy the contract** (new terminal — Ganache is ephemeral, all data is lost on restart):
>    ```bash
>    cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
>    truffle migrate --reset --network development
>    ```
>    Copy the new contract address from the output.
> 3. **Start the bridge** (Terminal 2):
>    ```bash
>    cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
>    go build -o bridge-server ./cmd/bridge/
>    ./bridge-server -contract 0x<NEW_CONTRACT_ADDRESS> -client http://127.0.0.1:7545
>    ```
> 4. **Start the Veriqid server** (Terminal 3):
>    ```bash
>    cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
>    go run ./cmd/veriqid-server \
>      -contract 0x<NEW_CONTRACT_ADDRESS> \
>      -service "KidsTube" \
>      -port 8080
>    ```
> 5. **Re-create identities** — old key files exist on disk but are not registered on the new contract. Either create fresh keys via the bridge API or re-run the test script.
>
> You will also need to use one of the Ganache private keys (without the `0x` prefix) for any operations that send Ethereum transactions (like creating identities or authorizing verifiers).

---

## BEFORE YOU START: Understanding What You're Building

In Phases 1–3, the server (`cmd/server/main.go`) was a minimal proof-of-concept:

1. It served static HTML files from `./static/`
2. It generated a challenge, injected it into an HTML form template via string replacement
3. It received proof values from the form, verified them, and showed a success/failure page
4. It had a **hardcoded** service name (`"abc_service"`, SHA-256 hashed)
5. **No persistence** — registered SPKs lived only in memory and vanished on restart
6. **No sessions** — each request was stateless, no way to track logged-in users
7. **No age verification** — the server didn't know or care about the child's age bracket
8. **No SDK endpoint** — other platforms couldn't programmatically verify proofs

Phase 4 transforms this into a **Veriqid-branded service demo** that shows what a real platform integration looks like. Think of it as a mock "KidsTube" — a platform that uses Veriqid for age-verified signups.

### What the Existing Server Does (Phase 1)

```
Browser → GET /directSignup → Server generates challenge, injects into signup.html
Browser → POST /signup       → Server receives {proof, spk, challenge}, verifies ring membership
Browser → GET /directLogin   → Server generates challenge, injects into login.html
Browser → POST /login        → Server receives {signature, spk, challenge}, verifies auth proof
```

**Problems:**

| Issue | Phase 1 | Phase 4 |
|-------|---------|---------|
| **Service name** | Hardcoded `"abc_service"` | Configurable via `-service` flag |
| **SPK storage** | In-memory array — lost on restart | SQLite database persists SPKs |
| **Sessions** | None — no way to know who's logged in | Cookie-based sessions with 1-hour expiry |
| **Age verification** | Not checked | Queries `Veriqid.sol` age bracket via contract |
| **API** | HTML forms only | JSON API for platform SDK integration |
| **Branding** | Generic "U2SSO" | Configurable platform name ("KidsTube") |
| **Challenge storage** | In-memory, not validated | Stored in SQLite with replay protection + 5-min expiry |
| **User accounts** | Don't persist | SPK → username mapping in SQLite |

### Why This Phase Matters

The browser extension (Phase 5) and demo platform (Phase 7) both need a **real server** to talk to. The extension auto-fills proofs into signup forms — but the server needs to actually verify those proofs, store the SPK, create a user session, and serve protected content. Without Phase 4, there's no "platform" for the extension to interact with.

The platform SDK verification endpoint (`POST /api/verify/registration` and `POST /api/verify/auth`) is also critical — it's what Phase 8 (JS SDK) will call from third-party platforms.

---

## PREREQUISITES

Before starting Phase 4, you must have Phases 1–3 fully complete:

```
[x] libsecp256k1 built and installed with --enable-module-ringcip
[x] go build ./cmd/client/ succeeds (CGO works)
[x] bridge/bridge.go compiles (go build -o bridge-server ./cmd/bridge/)
[x] Veriqid.sol deployed and Go bindings regenerated (Phase 3)
[x] Ganache running on port 7545
[x] Bridge API tested (Phase 2)
```

You'll also need:
- **SQLite3** development headers (`sudo apt install -y libsqlite3-dev`)
- A Ganache private key handy
- The contract address from your latest deployment

---

## STEP 1: Understand the Current Server Code

**Time: ~15 minutes (reading, no coding)**

### 1.1 The actual flow in cmd/server/main.go

Open the existing server file:

```bash
cat /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/cmd/server/main.go
```

The server has these key behaviors:

1. **Service name hashing** (hardcoded):
   ```go
   const sname = "abc_service"
   sha := sha256.New()
   sha.Write([]byte(sname))
   serviceName := hex.EncodeToString(sha.Sum(nil))
   ```
   The raw service name is SHA-256 hashed and hex-encoded. The client receives this pre-hashed hex — the client NEVER sees the raw name.

2. **Challenge generation** (inside `/directSignup` and `/directLogin` handlers):
   ```go
   challenge := u2sso.CreateChallenge()  // 32 random bytes via OpenSSL RAND_bytes
   ```

3. **Signup verification** (`/signup` handler):
   - Receives from form: `proof`, `spk`, `challenge`, `sname` (service name hex), `n` (ring size), `name` (username), `nullifier`
   - Fetches all active IDs from the contract
   - Calls `u2sso.RegistrationVerify()`

4. **Login verification** (`/login` handler):
   - Receives from form: `signature` (auth proof), `spk`, `challenge`, `sname`, `name`
   - Calls `u2sso.AuthVerify()`
   - Also checks in-memory array for previously registered SPKs

### 1.2 Key observations

- The service name is hashed **server-side** every time a form is loaded (same value each time since `sname` is constant).
- Challenges are **not stored** — anyone who intercepts a challenge can use it. No replay protection.
- SPKs from successful registrations are stored in an **in-memory slice** (`registeredSPK`) — lost on restart.
- The `RegistrationVerify()` and `AuthVerify()` functions come from `pkg/u2sso/u2ssolib.go` — they call the C library via CGO.

### 1.3 Form field names (critical — must match)

The existing static HTML forms use these field names:

| Field | Signup | Login |
|-------|--------|-------|
| Username | `name` | `name` |
| Challenge | `challenge` | `challenge` |
| Service name (hex) | `sname` | `sname` |
| SPK (hex) | `spk` | `spk` |
| Ring size | `n` | — |
| Proof | `proof` | — |
| Auth signature | — | `signature` |
| Nullifier | `nullifier` | — |

**Phase 4 preserves these exact field names** for backward compatibility with the browser extension and existing static HTML files.

### 1.4 What needs to change

| Component | Current | Target |
|-----------|---------|--------|
| Entry point | `cmd/server/main.go` | `cmd/veriqid-server/main.go` |
| Service name | Hardcoded `"abc_service"` | CLI flag `-service "KidsTube"` |
| Storage | In-memory slice | SQLite (`veriqid.db`) |
| Sessions | None | Standard library cookie sessions |
| API | HTML form handlers only | Additional JSON API endpoints |
| Age check | None | Default age bracket stored per user |
| Templates | Raw `static/` files with string replace | Go `html/template` with dynamic values |

---

## STEP 2: Install Dependencies

**Time: ~5 minutes**

### 2.1 Install SQLite3 development headers

```bash
sudo apt install -y libsqlite3-dev
```

### 2.2 Add Go dependencies

> **IMPORTANT — Go Toolchain Issue:** If you see `go: download go1.23 for linux/amd64: toolchain not available`, your `go.mod` requires a newer Go than what's installed. Fix with:
> ```bash
> export GOTOOLCHAIN=local
> ```
> Then if you see `go: go.mod requires go >= 1.23 (running go 1.22.2)`, downgrade the requirement:
> ```bash
> GOTOOLCHAIN=local go mod edit -go=1.22.2
> GOTOOLCHAIN=local go mod edit -toolchain=none
> ```

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

# SQLite driver (uses CGO internally — already enabled for libsecp256k1)
go get github.com/mattn/go-sqlite3

# UUID generation for session IDs
go get github.com/google/uuid

go mod tidy
```

> **NOTE — Do NOT install `gorilla/sessions`:** The latest `gorilla/sessions` v1.4.0 requires Go 1.23+. Phase 4 uses a **standard library session implementation** instead — zero external dependency, same functionality.

> **Verified versions from our build:**
> - `github.com/mattn/go-sqlite3 v1.14.34`
> - `github.com/google/uuid v1.6.0`

---

## STEP 3: Create the File Structure

**Time: ~5 minutes**

### 3.1 Create the new directories

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

mkdir -p cmd/veriqid-server
mkdir -p internal/store
mkdir -p internal/session
mkdir -p templates
```

Your directory structure will be:

```
Veriqid/
├── cmd/
│   ├── bridge/
│   │   └── main.go           ← Bridge entry point (Phase 2)
│   ├── client/
│   │   └── main.go           ← CLI tool (Phase 1)
│   ├── server/
│   │   └── main.go           ← Old server (Phase 1, kept for reference)
│   └── veriqid-server/
│       └── main.go           ← NEW: Veriqid server entry point
├── internal/
│   ├── store/
│   │   └── sqlite.go         ← NEW: SQLite storage layer
│   └── session/
│       └── session.go        ← NEW: Session management (standard library)
├── bridge/
│   └── bridge.go             ← Bridge handlers (Phase 2)
├── pkg/u2sso/
│   ├── u2ssolib.go           ← CGO crypto wrapper (Phase 1)
│   ├── U2SSO.go              ← Old contract bindings
│   └── Veriqid.go            ← New contract bindings (Phase 3)
├── templates/
│   ├── index.html            ← NEW: Dynamic home page
│   ├── signup.html           ← NEW: Dynamic signup form
│   ├── login.html            ← NEW: Dynamic login form
│   ├── dashboard.html        ← NEW: Post-login user dashboard
│   └── result.html           ← NEW: Success/failure page
├── static/                    ← CSS, images, JS (existing Phase 1 files)
└── ...
```

**Why `internal/`?** Go's `internal/` convention makes these packages importable only within the Veriqid module — they're private implementation details, not part of the public API.

---

## STEP 4: Build the SQLite Storage Layer

**Time: ~30 minutes**

### 4.1 Understanding the data model

The server needs to store:

1. **Registered users** — mapping SPK → username, with age bracket and registration timestamp
2. **Active challenges** — mapping challenge hex → metadata (timestamp, type), for replay protection

```sql
-- Users: SPK is the primary key (one user per SPK per platform)
CREATE TABLE IF NOT EXISTS users (
    spk_hex     TEXT PRIMARY KEY,      -- 66 chars (33-byte compressed point, hex-encoded)
    username    TEXT NOT NULL,
    age_bracket INTEGER DEFAULT 0,     -- 0=Unknown, 1=Under13, 2=Teen, 3=Adult
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login  DATETIME
);

-- Challenges: short-lived, cleaned up periodically
CREATE TABLE IF NOT EXISTS challenges (
    challenge_hex  TEXT PRIMARY KEY,    -- 64 chars (32 bytes, hex-encoded)
    challenge_type TEXT NOT NULL,       -- 'signup' or 'login'
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    used           BOOLEAN DEFAULT FALSE
);
```

### 4.2 Create `internal/store/sqlite.go`

```go
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
// Returns nil if not found.
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
```

### 4.3 Key design decisions

**Why SQLite and not in-memory?** The Phase 1 server lost all registered users on restart. SQLite persists across restarts — when Ganache gets restarted and the contract is redeployed, the server database can optionally be cleared too, but at least the server itself retains state between runs.

**Why one-time challenges?** Without this, an attacker who intercepts a challenge-response could replay it. By marking challenges as "used" in the database, each challenge can only be consumed once.

**Why a 5-minute expiry?** Challenges should be fresh. If a user loads the signup form, walks away for an hour, and comes back, the challenge should be expired. 5 minutes is generous enough for normal use but short enough to prevent stale challenges.

---

## STEP 5: Build the Session Management Layer

**Time: ~15 minutes**

> **IMPORTANT — Why Not `gorilla/sessions`?** The latest `gorilla/sessions` v1.4.0 requires Go 1.23+. Since our project runs on Go 1.22.2, and downgrading gorilla to v1.3.0 causes dependency conflicts with `go-ethereum`, we use a **standard library implementation** instead. It provides the exact same API (`Login`, `Logout`, `IsLoggedIn`) with zero external dependencies.

### 5.1 Create `internal/session/session.go`

```go
package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const SessionName = "veriqid-session"

// SessionData holds per-user session state.
type SessionData struct {
	SpkHex   string
	Username string
	LoggedIn bool
	Expiry   time.Time
}

// Manager stores sessions server-side keyed by an HMAC'd cookie ID.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionData
	secret   []byte
}

// NewManager creates a session manager with the given secret key.
func NewManager(secret []byte) *Manager {
	m := &Manager{
		sessions: make(map[string]*SessionData),
		secret:   secret,
	}
	// Cleanup expired sessions every 5 minutes
	go func() {
		for range time.Tick(5 * time.Minute) {
			m.cleanup()
		}
	}()
	return m
}

func (m *Manager) generateID() string {
	b := make([]byte, 32)
	rand.Read(b)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(b)
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// Login creates a session and sets the cookie.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request, spkHex, username string) error {
	sessionID := m.generateID()
	m.mu.Lock()
	m.sessions[sessionID] = &SessionData{
		SpkHex:   spkHex,
		Username: username,
		LoggedIn: true,
		Expiry:   time.Now().Add(1 * time.Hour),
	}
	m.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     SessionName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Logout clears the session and deletes the cookie.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(SessionName)
	if err == nil {
		m.mu.Lock()
		delete(m.sessions, cookie.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:   SessionName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	return nil
}

// IsLoggedIn checks if the user has an active session.
// Returns (loggedIn, spkHex, username).
func (m *Manager) IsLoggedIn(r *http.Request) (bool, string, string) {
	cookie, err := r.Cookie(SessionName)
	if err != nil {
		return false, "", ""
	}
	m.mu.RLock()
	sess, ok := m.sessions[cookie.Value]
	m.mu.RUnlock()
	if !ok || !sess.LoggedIn || time.Now().After(sess.Expiry) {
		return false, "", ""
	}
	return true, sess.SpkHex, sess.Username
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, sess := range m.sessions {
		if now.After(sess.Expiry) {
			delete(m.sessions, id)
		}
	}
}
```

### 5.2 How it works

- **Server-side storage** — session data lives in a `sync.RWMutex`-protected Go map, not in the cookie itself
- **HMAC'd session ID** — only a random HMAC'd token goes in the cookie; no user data is exposed
- **HttpOnly** — JavaScript can't read the cookie (XSS protection)
- **SameSite=Lax** — basic CSRF protection
- **1-hour expiry** — sessions auto-expire, cleaned up by background goroutine every 5 minutes

In production, you'd use server-side sessions (Redis) with just a session ID in the cookie. For the hackathon demo, in-memory sessions are sufficient.

---

## STEP 6: Create HTML Templates

**Time: ~20 minutes**

### 6.1 Understanding the template system

Instead of the Phase 1 approach of reading static HTML and doing `strings.Replace()`, the Veriqid server uses Go's `html/template` to inject dynamic values (service name, challenge, platform branding) into the pages at render time.

> **IMPORTANT — Form field names:** The templates use the **same form field names** as the Phase 1 static HTML (`name`, `spk`, `n`, `proof`, `sname`, `signature`, `nullifier`). This ensures backward compatibility with the browser extension (Phase 5) which detects these fields.

### 6.2 Create `templates/index.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.PlatformName}} — Powered by Veriqid</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container">
        <h1>Welcome to {{.PlatformName}}</h1>
        <p>Age-verified, privacy-preserving accounts powered by <strong>Veriqid</strong>.</p>

        {{if .LoggedIn}}
        <div class="user-info">
            <p>Logged in as: <strong>{{.Username}}</strong></p>
            <a href="/dashboard" class="btn btn-primary">Go to Dashboard</a>
            <a href="/logout" class="btn btn-secondary">Log Out</a>
        </div>
        {{else}}
        <div class="actions">
            <a href="/signup" class="btn btn-primary">Sign Up with Veriqid</a>
            <a href="/login" class="btn btn-secondary">Log In with Veriqid</a>
        </div>
        {{end}}

        <div class="info">
            <h3>How it works</h3>
            <p>Your identity is verified once through a trusted authority.
               No personal data is shared with {{.PlatformName}}.
               We only learn that you are a verified user — nothing else.</p>
        </div>
    </div>
</body>
</html>
```

### 6.3 Create `templates/signup.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign Up — {{.PlatformName}}</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container">
        <h1>Sign Up for {{.PlatformName}}</h1>
        <p>Prove your age-verified identity using Veriqid.</p>

        <form method="POST" action="/signup" id="signup-form">
            <input type="hidden" name="challenge" value="{{.Challenge}}" id="veriqid-challenge">
            <input type="hidden" name="sname" value="{{.ServiceName}}" id="veriqid-service-name">

            <div class="form-group">
                <label for="name">Choose a username:</label>
                <input type="text" name="name" id="name" required
                       placeholder="e.g., coolkid42" maxlength="30">
            </div>

            <div class="form-group">
                <label for="spk">Public Key (SPK):</label>
                <input type="text" name="spk" id="veriqid-spk" required
                       placeholder="Will be filled by Veriqid extension">
            </div>

            <div class="form-group">
                <label for="n">Ring Size (N):</label>
                <input type="text" name="n" id="veriqid-ring-size" required
                       placeholder="Will be filled by Veriqid extension">
            </div>

            <div class="form-group">
                <label for="proof">Membership Proof:</label>
                <textarea name="proof" id="veriqid-proof" required rows="4"
                          placeholder="Will be filled by Veriqid extension"></textarea>
            </div>

            <input type="hidden" name="nullifier" value="">

            <button type="submit" class="btn btn-primary">Sign Up!</button>
        </form>

        <p class="note">
            <strong>Challenge:</strong> <code>{{.Challenge}}</code><br>
            <strong>Service:</strong> <code>{{.ServiceName}}</code>
        </p>
    </div>
</body>
</html>
```

### 6.4 Create `templates/login.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Log In — {{.PlatformName}}</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container">
        <h1>Log In to {{.PlatformName}}</h1>
        <p>Authenticate using your Veriqid identity.</p>

        <form method="POST" action="/login" id="login-form">
            <input type="hidden" name="challenge" value="{{.Challenge}}" id="veriqid-challenge">
            <input type="hidden" name="sname" value="{{.ServiceName}}" id="veriqid-service-name">

            <div class="form-group">
                <label for="name">Username:</label>
                <input type="text" name="name" id="name" required
                       placeholder="Your username">
            </div>

            <div class="form-group">
                <label for="spk">Public Key (SPK):</label>
                <input type="text" name="spk" id="veriqid-spk" required
                       placeholder="Will be filled by Veriqid extension">
            </div>

            <div class="form-group">
                <label for="signature">Auth Proof:</label>
                <textarea name="signature" id="veriqid-auth-proof" required rows="4"
                          placeholder="Will be filled by Veriqid extension"></textarea>
            </div>

            <button type="submit" class="btn btn-primary">Log In!</button>
        </form>

        <p class="note">
            <strong>Challenge:</strong> <code>{{.Challenge}}</code><br>
            <strong>Service:</strong> <code>{{.ServiceName}}</code>
        </p>
    </div>
</body>
</html>
```

### 6.5 Create `templates/dashboard.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Dashboard — {{.PlatformName}}</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container">
        <h1>Welcome, {{.Username}}!</h1>
        <p>You are logged in to <strong>{{.PlatformName}}</strong>.</p>

        <div class="dashboard-info">
            <h3>Your Account</h3>
            <p><strong>Username:</strong> {{.Username}}</p>
            <p><strong>Age Bracket:</strong> {{.AgeBracketLabel}}</p>
            <p><strong>Registered:</strong> {{.CreatedAt}}</p>
            <p><strong>Last Login:</strong> {{.LastLogin}}</p>
        </div>

        <div class="dashboard-content">
            <h3>Platform Content</h3>
            <p>This is where {{.PlatformName}} would show age-appropriate content.
               Because your age bracket is <strong>{{.AgeBracketLabel}}</strong>,
               content is filtered accordingly.</p>
        </div>

        <a href="/logout" class="btn btn-secondary">Log Out</a>
    </div>
</body>
</html>
```

### 6.6 Create `templates/result.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} — {{.PlatformName}}</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container">
        <h1>{{.Title}}</h1>
        <p>{{.Message}}</p>
        <a href="/" class="btn btn-primary">Back to Home</a>
    </div>
</body>
</html>
```

---

## STEP 7: Write the Veriqid Server Core

**Time: ~45 minutes**

### 7.1 Create `cmd/veriqid-server/main.go`

> **NOTE — CGO import placement:** The CGO preamble (`// #cgo ...` and `import "C"`) is placed AFTER the Go import block, matching the exact pattern used in `cmd/server/main.go`. This is valid Go — the CGO preamble just needs to be immediately above `import "C"` with no blank line between the comment and the import.

> **NOTE — `bytes` import:** The `bytes` package is imported (and kept alive via `var _ = bytes.Compare`) for future expansion. The Phase 1 server used `bytes.Compare` for SPK matching.

```go
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	u2sso "github.com/patmekury/veriqid/pkg/u2sso"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/patmekury/veriqid/internal/session"
	"github.com/patmekury/veriqid/internal/store"
)

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lcrypto -lsecp256k1
// #include <stdlib.h>
// #include <stdint.h>
// #include <string.h>
// #include <openssl/rand.h>
// #include <secp256k1.h>
// #include <secp256k1_ringcip.h>
import "C"
```

### 7.2 The Server struct

```go
// Server holds all the state for the Veriqid platform server.
type Server struct {
	PlatformName   string
	ServiceNameHex string // SHA-256 hash of the service name, hex-encoded
	ServiceNameRaw []byte // SHA-256 hash bytes (for passing to verification functions)
	ContractAddr   string
	RPCURL         string

	Store    *store.Store
	Sessions *session.Manager
	Tmpl     *template.Template

	EthClient *ethclient.Client
	Contract  *u2sso.Veriqid
}
```

### 7.3 Constructor

```go
func NewServer(platformName, contractAddr, rpcURL, dbPath string) (*Server, error) {
	// Hash the service name (same as Phase 1 server)
	h := sha256.Sum256([]byte(platformName))
	serviceHex := hex.EncodeToString(h[:])

	// Open SQLite database
	db, err := store.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Session manager — random 32-byte key
	sessionKey := make([]byte, 32)
	rand.Read(sessionKey)
	sessions := session.NewManager(sessionKey)

	// Parse templates
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Connect to Ethereum
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum at %s: %w", rpcURL, err)
	}

	// Verify contract exists
	address := common.HexToAddress(contractAddr)
	bytecode, err := client.CodeAt(context.Background(), address, nil)
	if err != nil || len(bytecode) == 0 {
		return nil, fmt.Errorf("no contract found at %s", contractAddr)
	}

	// Create contract instance (using Phase 3 Veriqid bindings)
	contract, err := u2sso.NewVeriqid(address, client)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate Veriqid contract: %w", err)
	}

	return &Server{
		PlatformName:   platformName,
		ServiceNameHex: serviceHex,
		ServiceNameRaw: h[:],
		ContractAddr:   contractAddr,
		RPCURL:         rpcURL,
		Store:          db,
		Sessions:       sessions,
		Tmpl:           tmpl,
		EthClient:      client,
		Contract:       contract,
	}, nil
}
```

### 7.4 Helper functions

```go
func ageBracketLabel(bracket int) string {
	switch bracket {
	case 1:
		return "Under 13"
	case 2:
		return "Teen (13-17)"
	case 3:
		return "Adult (18+)"
	default:
		return "Unknown"
	}
}

func (s *Server) renderResult(w http.ResponseWriter, title, message string, statusCode int) {
	w.WriteHeader(statusCode)
	s.Tmpl.ExecuteTemplate(w, "result.html", map[string]interface{}{
		"PlatformName": s.PlatformName,
		"Title":        title,
		"Message":      message,
	})
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   true,
		"message": message,
	})
}

// Keep bytes import alive for future use
var _ = bytes.Compare
```

---

## STEP 8: Implement the Route Handlers

**Time: ~45 minutes**

### 8.1 Home page handler

```go
func (s *Server) HandleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	loggedIn, _, username := s.Sessions.IsLoggedIn(r)

	s.Tmpl.ExecuteTemplate(w, "index.html", map[string]interface{}{
		"PlatformName": s.PlatformName,
		"LoggedIn":     loggedIn,
		"Username":     username,
	})
}
```

### 8.2 Signup form handler (GET)

```go
func (s *Server) HandleSignupForm(w http.ResponseWriter, r *http.Request) {
	challenge := u2sso.CreateChallenge()
	challengeHex := hex.EncodeToString(challenge)

	if err := s.Store.StoreChallenge(challengeHex, "signup"); err != nil {
		log.Printf("ERROR: failed to store challenge: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Tmpl.ExecuteTemplate(w, "signup.html", map[string]interface{}{
		"PlatformName": s.PlatformName,
		"Challenge":    challengeHex,
		"ServiceName":  s.ServiceNameHex,
	})
}
```

### 8.3 Signup submission handler (POST)

This is the most complex handler — it verifies the ring membership proof, checks the SPK isn't already registered, and creates the user.

```go
func (s *Server) HandleSignupSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.ParseForm(); err != nil {
		s.renderResult(w, "Error", "Failed to parse form data.", http.StatusBadRequest)
		return
	}

	// Extract form values — using the same field names as Phase 1
	username := strings.TrimSpace(r.FormValue("name"))
	challengeHex := r.FormValue("challenge")
	spkHex := r.FormValue("spk")
	proofHex := r.FormValue("proof")
	ringSizeStr := r.FormValue("n")
	serviceNameHex := r.FormValue("sname")

	if challengeHex == "" || spkHex == "" || proofHex == "" || ringSizeStr == "" || username == "" {
		s.renderResult(w, "Sign Up Failed", "All fields are required.", http.StatusBadRequest)
		return
	}

	// Validate challenge (replay protection)
	valid, err := s.Store.ValidateChallenge(challengeHex, "signup")
	if err != nil {
		log.Printf("ERROR: challenge validation failed: %v", err)
		s.renderResult(w, "Error", "Internal server error.", http.StatusInternalServerError)
		return
	}
	if !valid {
		s.renderResult(w, "Sign Up Failed", "Challenge expired or already used. Please try again.", http.StatusBadRequest)
		return
	}

	// Check if this SPK is already registered
	exists, err := s.Store.UserExists(spkHex)
	if err != nil {
		log.Printf("ERROR: user check failed: %v", err)
		s.renderResult(w, "Error", "Internal server error.", http.StatusInternalServerError)
		return
	}
	if exists {
		s.renderResult(w, "Sign Up Failed", "This identity is already registered on this platform.", http.StatusConflict)
		return
	}

	// Decode hex values
	challenge, _ := hex.DecodeString(challengeHex)
	spkBytes, _ := hex.DecodeString(spkHex)
	serviceName, _ := hex.DecodeString(serviceNameHex)

	ringSize, err := strconv.Atoi(ringSizeStr)
	if err != nil || ringSize < 2 {
		s.renderResult(w, "Sign Up Failed", "Invalid ring size.", http.StatusBadRequest)
		return
	}

	// Calculate ring parameters (same logic as bridge and CLI)
	currentm := 1
	tmp := 1
	for currentm = 1; currentm < u2sso.M; currentm++ {
		tmp *= u2sso.N
		if tmp >= ringSize {
			break
		}
	}

	// Fetch all active IDs from the contract
	idList, err := u2sso.GetallActiveIDfromContract(s.Contract)
	if err != nil {
		log.Printf("ERROR: failed to fetch IDs from contract: %v", err)
		s.renderResult(w, "Error", "Failed to fetch identity ring from blockchain.", http.StatusInternalServerError)
		return
	}

	// Verify the ring membership proof
	verified := u2sso.RegistrationVerify(proofHex, currentm, ringSize, serviceName, challenge, idList, spkBytes)

	if !verified {
		s.renderResult(w, "Sign Up Failed", "Membership proof verification failed.", http.StatusForbidden)
		return
	}

	// Default age bracket: Under 13 (the primary use case)
	ageBracket := 1

	// Register the user in the database
	if err := s.Store.RegisterUser(spkHex, username, ageBracket); err != nil {
		log.Printf("ERROR: failed to register user: %v", err)
		s.renderResult(w, "Sign Up Failed", "Registration failed.", http.StatusConflict)
		return
	}

	// Create a session
	if err := s.Sessions.Login(w, r, spkHex, username); err != nil {
		log.Printf("ERROR: failed to create session: %v", err)
	}

	log.Printf("SUCCESS: User '%s' registered with SPK %s...", username, spkHex[:16])
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
```

### 8.4 Login form handler (GET)

```go
func (s *Server) HandleLoginForm(w http.ResponseWriter, r *http.Request) {
	challenge := u2sso.CreateChallenge()
	challengeHex := hex.EncodeToString(challenge)

	if err := s.Store.StoreChallenge(challengeHex, "login"); err != nil {
		log.Printf("ERROR: failed to store challenge: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{
		"PlatformName": s.PlatformName,
		"Challenge":    challengeHex,
		"ServiceName":  s.ServiceNameHex,
	})
}
```

### 8.5 Login submission handler (POST)

```go
func (s *Server) HandleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.ParseForm(); err != nil {
		s.renderResult(w, "Error", "Failed to parse form data.", http.StatusBadRequest)
		return
	}

	challengeHex := r.FormValue("challenge")
	spkHex := r.FormValue("spk")
	signatureHex := r.FormValue("signature") // auth proof
	serviceNameHex := r.FormValue("sname")
	_ = r.FormValue("name") // username from form (display only)

	if challengeHex == "" || spkHex == "" || signatureHex == "" {
		s.renderResult(w, "Login Failed", "All fields are required.", http.StatusBadRequest)
		return
	}

	// Validate challenge (replay protection)
	valid, err := s.Store.ValidateChallenge(challengeHex, "login")
	if err != nil || !valid {
		s.renderResult(w, "Login Failed", "Challenge expired or already used. Please try again.", http.StatusBadRequest)
		return
	}

	// Check if this SPK is registered on this platform
	user, err := s.Store.GetUserBySPK(spkHex)
	if err != nil {
		log.Printf("ERROR: user lookup failed: %v", err)
		s.renderResult(w, "Error", "Internal server error.", http.StatusInternalServerError)
		return
	}
	if user == nil {
		s.renderResult(w, "Login Failed", "No account found for this identity. Please sign up first.", http.StatusNotFound)
		return
	}

	// Decode and verify
	challenge, _ := hex.DecodeString(challengeHex)
	spkBytes, _ := hex.DecodeString(spkHex)
	serviceName, _ := hex.DecodeString(serviceNameHex)

	verified := u2sso.AuthVerify(signatureHex, serviceName, challenge, spkBytes)

	if !verified {
		s.renderResult(w, "Login Failed", "Authentication proof verification failed.", http.StatusForbidden)
		return
	}

	// Update last login and create session
	s.Store.UpdateLastLogin(spkHex)
	s.Sessions.Login(w, r, spkHex, user.Username)

	log.Printf("SUCCESS: User '%s' logged in", user.Username)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
```

### 8.6 Dashboard handler

```go
func (s *Server) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	loggedIn, spkHex, username := s.Sessions.IsLoggedIn(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := s.Store.GetUserBySPK(spkHex)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	s.Tmpl.ExecuteTemplate(w, "dashboard.html", map[string]interface{}{
		"PlatformName":    s.PlatformName,
		"Username":        username,
		"AgeBracketLabel": ageBracketLabel(user.AgeBracket),
		"CreatedAt":       user.CreatedAt.Format("January 2, 2006"),
		"LastLogin":       user.LastLogin.Format("January 2, 2006 3:04 PM"),
	})
}
```

### 8.7 Logout handler

```go
func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	s.Sessions.Logout(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

---

## STEP 9: Add the Platform SDK Verification API

**Time: ~25 minutes**

### 9.1 Why a JSON API?

The HTML form handlers are for human users in a browser. The JSON API is for **programmatic verification** — other platforms can send proof data via their backend and get a verification result. This is what the Phase 8 JS SDK will call.

### 9.2 API: GET /api/challenge

```go
type APIChallengeResponse struct {
	Challenge   string `json:"challenge"`
	ServiceName string `json:"service_name"`
}

func (s *Server) HandleAPIChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "use GET")
		return
	}

	challenge := u2sso.CreateChallenge()
	challengeHex := hex.EncodeToString(challenge)

	challengeType := r.URL.Query().Get("type")
	if challengeType != "login" {
		challengeType = "signup"
	}

	s.Store.StoreChallenge(challengeHex, challengeType)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIChallengeResponse{
		Challenge:   challengeHex,
		ServiceName: s.ServiceNameHex,
	})
}
```

### 9.3 API: POST /api/verify/registration

```go
type VerifyRegistrationRequest struct {
	ProofHex     string `json:"proof_hex"`
	SpkHex       string `json:"spk_hex"`
	ChallengeHex string `json:"challenge_hex"`
	RingSize     int    `json:"ring_size"`
	ServiceName  string `json:"service_name"` // Pre-hashed hex (optional)
}

type VerifyRegistrationResponse struct {
	Verified   bool   `json:"verified"`
	Message    string `json:"message,omitempty"`
	AgeBracket int    `json:"age_bracket,omitempty"`
}

func (s *Server) HandleAPIVerifyRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	var req VerifyRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.ProofHex == "" || req.SpkHex == "" || req.ChallengeHex == "" || req.RingSize < 2 {
		writeAPIError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	var serviceNameBytes []byte
	if req.ServiceName != "" {
		serviceNameBytes, _ = hex.DecodeString(req.ServiceName)
	} else {
		serviceNameBytes = s.ServiceNameRaw
	}

	challenge, _ := hex.DecodeString(req.ChallengeHex)
	spkBytes, _ := hex.DecodeString(req.SpkHex)

	currentm := 1
	tmp := 1
	for currentm = 1; currentm < u2sso.M; currentm++ {
		tmp *= u2sso.N
		if tmp >= req.RingSize {
			break
		}
	}

	idList, err := u2sso.GetallActiveIDfromContract(s.Contract)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to fetch identity ring")
		return
	}

	verified := u2sso.RegistrationVerify(
		req.ProofHex, currentm, req.RingSize,
		serviceNameBytes, challenge, idList, spkBytes,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VerifyRegistrationResponse{
		Verified:   verified,
		AgeBracket: 1,
	})
}
```

### 9.4 API: POST /api/verify/auth

```go
type VerifyAuthRequest struct {
	AuthProofHex string `json:"auth_proof_hex"`
	SpkHex       string `json:"spk_hex"`
	ChallengeHex string `json:"challenge_hex"`
	ServiceName  string `json:"service_name"`
}

type VerifyAuthResponse struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message,omitempty"`
}

func (s *Server) HandleAPIVerifyAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	var req VerifyAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.AuthProofHex == "" || req.SpkHex == "" || req.ChallengeHex == "" {
		writeAPIError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	var serviceNameBytes []byte
	if req.ServiceName != "" {
		serviceNameBytes, _ = hex.DecodeString(req.ServiceName)
	} else {
		serviceNameBytes = s.ServiceNameRaw
	}

	challenge, _ := hex.DecodeString(req.ChallengeHex)
	spkBytes, _ := hex.DecodeString(req.SpkHex)

	verified := u2sso.AuthVerify(req.AuthProofHex, serviceNameBytes, challenge, spkBytes)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VerifyAuthResponse{
		Verified: verified,
	})
}
```

### 9.5 API: GET /api/status

```go
type APIStatusResponse struct {
	Status      string `json:"status"`
	Platform    string `json:"platform"`
	ServiceName string `json:"service_name"`
	Contract    string `json:"contract"`
	UserCount   int    `json:"user_count"`
	Version     string `json:"version"`
}

func (s *Server) HandleAPIStatus(w http.ResponseWriter, r *http.Request) {
	userCount, _ := s.Store.GetUserCount()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIStatusResponse{
		Status:      "ok",
		Platform:    s.PlatformName,
		ServiceName: s.ServiceNameHex,
		Contract:    s.ContractAddr,
		UserCount:   userCount,
		Version:     "0.4.0",
	})
}
```

---

## STEP 10: Wire Up the Router and Main Function

**Time: ~15 minutes**

### 10.1 Route registration

```go
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// HTML page routes
	mux.HandleFunc("/", s.HandleHome)
	mux.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.HandleSignupForm(w, r)
		} else {
			s.HandleSignupSubmit(w, r)
		}
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.HandleLoginForm(w, r)
		} else {
			s.HandleLoginSubmit(w, r)
		}
	})
	mux.HandleFunc("/dashboard", s.HandleDashboard)
	mux.HandleFunc("/logout", s.HandleLogout)

	// Backward-compatible routes (Phase 1 HTML form endpoints)
	mux.HandleFunc("/directSignup", s.HandleSignupForm)
	mux.HandleFunc("/directLogin", s.HandleLoginForm)

	// JSON API routes (for platform SDK integration)
	mux.HandleFunc("/api/status", s.HandleAPIStatus)
	mux.HandleFunc("/api/challenge", s.HandleAPIChallenge)
	mux.HandleFunc("/api/verify/registration", s.HandleAPIVerifyRegistration)
	mux.HandleFunc("/api/verify/auth", s.HandleAPIVerifyAuth)
}
```

### 10.2 CORS middleware (for API endpoints)

```go
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
```

### 10.3 Background cleanup goroutine

```go
func startCleanupWorker(db *store.Store) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			if err := db.CleanExpiredChallenges(); err != nil {
				log.Printf("WARNING: challenge cleanup failed: %v", err)
			}
		}
	}()
}
```

### 10.4 Main function

```go
func main() {
	contractAddr := flag.String("contract", "", "Veriqid smart contract address (required)")
	clientAddr := flag.String("client", "http://127.0.0.1:7545", "Ethereum JSON-RPC endpoint")
	port := flag.Int("port", 8080, "Port for the server to listen on")
	serviceName := flag.String("service", "KidsTube", "Platform service name (used for SPK derivation)")
	dbPath := flag.String("db", "./veriqid.db", "Path to SQLite database file")
	flag.Parse()

	if *contractAddr == "" {
		log.Fatal("Error: -contract flag is required.\n\nUsage:\n  ./veriqid-server -contract 0x... [-service KidsTube] [-port 8080] [-db ./veriqid.db]")
	}

	srv, err := NewServer(*serviceName, *contractAddr, *clientAddr, *dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}
	defer srv.Store.Close()

	startCleanupWorker(srv.Store)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	handler := CORSMiddleware(mux)

	addr := fmt.Sprintf(":%d", *port)
	fmt.Println("===========================================")
	fmt.Printf("  %s — Powered by Veriqid\n", *serviceName)
	fmt.Println("===========================================")
	fmt.Printf("  Server:     http://localhost:%d\n", *port)
	fmt.Printf("  Service:    %s\n", *serviceName)
	fmt.Printf("  Contract:   %s\n", *contractAddr)
	fmt.Printf("  RPC:        %s\n", *clientAddr)
	fmt.Printf("  Database:   %s\n", *dbPath)
	fmt.Println("-------------------------------------------")
	fmt.Println("  Pages:")
	fmt.Println("    GET  /             Home page")
	fmt.Println("    GET  /signup       Signup form")
	fmt.Println("    POST /signup       Submit signup")
	fmt.Println("    GET  /login        Login form")
	fmt.Println("    POST /login        Submit login")
	fmt.Println("    GET  /dashboard    User dashboard")
	fmt.Println("    GET  /logout       Log out")
	fmt.Println("    GET  /directSignup Phase 1 compat")
	fmt.Println("    GET  /directLogin  Phase 1 compat")
	fmt.Println("  API:")
	fmt.Println("    GET  /api/status              Server status")
	fmt.Println("    GET  /api/challenge            Generate challenge")
	fmt.Println("    POST /api/verify/registration  Verify registration proof")
	fmt.Println("    POST /api/verify/auth          Verify auth proof")
	fmt.Println("===========================================")

	log.Fatal(http.ListenAndServe(addr, handler))
}
```

---

## STEP 11: Build and Test

**Time: ~20 minutes**

### 11.1 Build the binary

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

go build -o veriqid-server ./cmd/veriqid-server/
```

**If it fails with import errors:** Run `go mod tidy`. If you see `go.mod requires go >= 1.23`, run:
```bash
export GOTOOLCHAIN=local
go mod edit -go=1.22.2
go mod edit -toolchain=none
go build -o veriqid-server ./cmd/veriqid-server/
```

### 11.2 Start the prerequisites

You need three terminals running:

**Terminal 1 — Ganache:**
```bash
ganache --port 7545
```

**Terminal 2 — Deploy contract and start bridge:**
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle migrate --reset --network development
# Copy the contract address, e.g., 0x37e763BAf75360...

cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go build -o bridge-server ./cmd/bridge/
./bridge-server -contract 0x<YOUR_CONTRACT_ADDRESS> -client http://127.0.0.1:7545
```

**Terminal 3 — Start the Veriqid server:**
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

./veriqid-server -contract 0x<YOUR_CONTRACT_ADDRESS> -service "KidsTube" -port 8080
```

You should see the startup banner:
```
===========================================
  KidsTube — Powered by Veriqid
===========================================
  Server:     http://localhost:8080
  Service:    KidsTube
  Contract:   0x37e763BAf75360...
  RPC:        http://127.0.0.1:7545
  Database:   ./veriqid.db
-------------------------------------------
```

### 11.3 Test the web interface

Open your browser to `http://localhost:8080`. You should see the branded home page with "Welcome to KidsTube" and signup/login buttons.

### 11.4 Test the API endpoints

```bash
# Test status
curl -s http://localhost:8080/api/status | python3 -m json.tool

# Test challenge generation
curl -s "http://localhost:8080/api/challenge?type=signup" | python3 -m json.tool
```

### 11.5 Test the full signup flow (requires bridge + identities)

```bash
# Step 1: Get a challenge
CHALLENGE=$(curl -s "http://localhost:8080/api/challenge?type=signup" | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])")
SERVICE=$(curl -s http://localhost:8080/api/status | python3 -c "import sys,json; print(json.load(sys.stdin)['service_name'])")

# Step 2: Generate proof via bridge
PROOF_RESPONSE=$(curl -s -X POST http://localhost:9090/api/identity/register \
  -H "Content-Type: application/json" \
  -d "{\"keypath\": \"./key1\", \"service_name\": \"$SERVICE\", \"challenge\": \"$CHALLENGE\"}")

# Step 3: Extract values
PROOF_HEX=$(echo $PROOF_RESPONSE | python3 -c "import sys,json; print(json.load(sys.stdin)['proof_hex'])")
SPK_HEX=$(echo $PROOF_RESPONSE | python3 -c "import sys,json; print(json.load(sys.stdin)['spk_hex'])")
RING_SIZE=$(echo $PROOF_RESPONSE | python3 -c "import sys,json; print(json.load(sys.stdin)['ring_size'])")

# Step 4: Verify via server API
curl -s -X POST http://localhost:8080/api/verify/registration \
  -H "Content-Type: application/json" \
  -d "{\"proof_hex\":\"$PROOF_HEX\",\"spk_hex\":\"$SPK_HEX\",\"challenge_hex\":\"$CHALLENGE\",\"ring_size\":$RING_SIZE}" | python3 -m json.tool
```

> **IMPORTANT:** The shell variables (`$PROOF_HEX`, etc.) must be set in the **same terminal session** where you run the verify command. If they're empty, the JSON will be malformed and you'll get `invalid character '}' looking for beginning of value`.

---

## STEP 12: Run the Automated Test Script

**Time: ~5 minutes**

### 12.1 Run `test_veriqid_server.sh`

The test script is included at `Veriqid/test_veriqid_server.sh`. It runs 11 test suites covering:

1. `GET /api/status` — platform info and version
2. `GET /api/challenge` — challenge generation (64 hex chars)
3. Identity creation via bridge
4. Registration proof generation via bridge
5. `POST /api/verify/registration` — proof verification
6. Auth proof generation via bridge
7. `POST /api/verify/auth` — auth verification
8. Error handling — empty body, wrong method, invalid JSON
9. HTML page status codes — `/`, `/signup`, `/login` return 200, `/dashboard` redirects when not logged in
10. Phase 1 backward-compatible routes — `/directSignup`, `/directLogin`
11. CORS headers on API endpoints

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

# Fix Windows line endings if needed
sed -i 's/\r$//' test_veriqid_server.sh
chmod +x test_veriqid_server.sh

# Run (use a Ganache private key without 0x prefix)
./test_veriqid_server.sh <GANACHE_PRIVATE_KEY_WITHOUT_0x>
```

**Expected output (all passed):**

```
=== Pre-flight: Checking services ===
  ✅ PASS: Veriqid server is running
  ✅ PASS: Bridge is running

=== Test 1: GET /api/status ===
  ✅ PASS: status == 'ok'
  ✅ PASS: platform name present
  ✅ PASS: service_name is 64 hex chars (SHA-256)
  ✅ PASS: version == 0.4.0

...

===========================================
  Results: XX passed, 0 failed, 0 skipped
  🎉 ALL TESTS PASSED
===========================================
```

---

## STEP 13: Troubleshooting

### 13.1 Common errors

| Error | Cause | Fix |
|-------|-------|-----|
| `failed to parse templates: pattern matches no files` | Templates directory missing or wrong working directory | Run the server from the Veriqid root directory where `templates/` exists |
| `no contract found at address 0x...` | Ganache was restarted | Redeploy with `truffle migrate --reset`, restart server with new address |
| `failed to open database` | SQLite permission issue | Check directory write permissions, or use a different `-db` path |
| `template: "index.html" not defined` | Template file name mismatch | Ensure template filenames match the `ExecuteTemplate()` calls |
| `cannot find package "github.com/mattn/go-sqlite3"` | Dependencies not installed | Run `go get github.com/mattn/go-sqlite3 && go mod tidy` |
| `build constraints exclude all Go files in .../go-sqlite3` | CGO disabled | Ensure `CGO_ENABLED=1` (run `go env CGO_ENABLED`) |
| `go: download go1.23 for linux/amd64: toolchain not available` | Go toolchain version mismatch | Run `export GOTOOLCHAIN=local` then `go mod edit -go=1.22.2 && go mod edit -toolchain=none` |
| `module github.com/gorilla/sessions@v1.4.0 requires go >= 1.23` | gorilla/sessions in go.mod | Run `go mod edit -droprequire=github.com/gorilla/sessions` — Phase 4 does NOT use gorilla |
| `invalid character '}' looking for beginning of value` | Empty shell variables in curl JSON | Ensure `$PROOF_HEX`, `$SPK_HEX`, etc. are set in the same terminal session |
| `-bash: ./test_veriqid_server.sh: cannot execute: required file not found` | Windows line endings (CRLF) | Run `sed -i 's/\r$//' test_veriqid_server.sh` |
| Session cookie not being set | Browser blocking localhost cookies | Use `127.0.0.1:8080` instead of `localhost:8080` |

### 13.2 Database inspection

```bash
# View all registered users
sqlite3 veriqid.db "SELECT username, age_bracket, created_at FROM users;"

# View active challenges
sqlite3 veriqid.db "SELECT challenge_hex, challenge_type, used, created_at FROM challenges ORDER BY created_at DESC LIMIT 10;"

# Count users
sqlite3 veriqid.db "SELECT COUNT(*) FROM users;"

# Reset the database (start fresh)
rm veriqid.db
# The server will recreate it on next start
```

---

## STEP 14: Phase 4 Completion Checklist

```
[x] internal/store/sqlite.go created with user and challenge tables
[x] internal/session/session.go created with standard library cookie sessions (no gorilla)
[x] Templates created: index.html, signup.html, login.html, dashboard.html, result.html
[x] cmd/veriqid-server/main.go compiles (go build -o veriqid-server ./cmd/veriqid-server/)
[x] Server starts and shows branded startup banner
[x] GET / shows the branded home page ("Welcome to KidsTube")
[x] GET /signup shows the signup form with fresh challenge
[x] POST /signup verifies proof, creates user in SQLite, creates session
[x] GET /login shows the login form with fresh challenge
[x] POST /login verifies auth proof, checks SPK exists, creates session
[x] GET /dashboard shows user info (requires active session)
[x] GET /logout clears the session
[x] GET /api/status returns JSON with platform info and user count
[x] GET /api/challenge returns challenge + service_name
[x] POST /api/verify/registration verifies a registration proof (JSON API)
[x] POST /api/verify/auth verifies an auth proof (JSON API)
[x] Challenges are one-time use (replay protection)
[x] Challenges expire after 5 minutes
[x] Session cookies are HttpOnly and have 1-hour expiry
[x] CORS headers are set on /api/* endpoints
[x] Service name is configurable via -service flag
[x] SQLite database persists across server restarts
[x] Backward-compatible routes: /directSignup, /directLogin
[x] test_veriqid_server.sh passes all tests
```

---

## What Each New File Does (Reference)

### cmd/veriqid-server/main.go (~480 lines)
The Veriqid server entry point. Parses CLI flags (`-contract`, `-service`, `-port`, `-db`, `-client`), initializes the server with SQLite store, session manager, template engine, and contract connection. Registers all routes (including Phase 1 backward-compatible `/directSignup` and `/directLogin`), starts background challenge cleanup worker, listens on the specified port. Includes all route handlers for both HTML pages and JSON API.

### internal/store/sqlite.go (~180 lines)
SQLite storage layer. Manages two tables: `users` (SPK → username mapping with age bracket and timestamps) and `challenges` (one-time challenges with expiry). Provides methods for user registration, lookup, challenge storage and validation, and expired challenge cleanup.

### internal/session/session.go (~110 lines)
Session management using the **Go standard library** (no gorilla/sessions — it requires Go 1.23+). Session data stored server-side in a `sync.RWMutex`-protected map. Only an HMAC'd session ID goes in the cookie. Handles login (set session), logout (clear session), and `IsLoggedIn` checks. Cookies have 1-hour expiry, HttpOnly, and SameSite=Lax. Background goroutine cleans up expired sessions every 5 minutes.

### templates/*.html (5 files)
Go HTML templates with dynamic values. Each template receives a map of values (platform name, challenge, service name, user info) at render time. The signup and login forms use the **same field names as Phase 1** (`name`, `spk`, `n`, `proof`, `sname`, `signature`, `nullifier`) and include hidden fields with IDs that the browser extension (Phase 5) will detect and auto-fill (`veriqid-challenge`, `veriqid-service-name`, `veriqid-spk`, `veriqid-proof`, `veriqid-ring-size`, `veriqid-auth-proof`).

### test_veriqid_server.sh (~200 lines)
End-to-end test script covering 11 test suites. Pre-flight checks that both the server and bridge are running. Tests API endpoints, registration/auth proof verification, error handling, HTML page status codes, backward-compatible routes, and CORS headers.

---

## How the Server Connects to Other Phases

### Phase 5 (Browser Extension)
The extension detects Veriqid form fields on the signup/login pages by looking for elements with IDs like `veriqid-challenge`, `veriqid-spk`, `veriqid-proof`. When it finds them, it:
1. Reads the challenge and service name from the hidden fields
2. Calls the bridge API to generate the proof
3. Auto-fills the form fields with the proof values
4. Optionally auto-submits the form

### Phase 7 (Demo Platform)
The demo platform ("KidsTube") IS this server running with `-service "KidsTube"`. Phase 7 adds polished UI, video content placeholders, and a more realistic platform experience — but the backend verification logic is all here in Phase 4.

### Phase 8 (JS SDK)
The JS SDK provides a `Veriqid.verifyRegistration()` function that calls `POST /api/verify/registration` and a `Veriqid.verifyAuth()` function that calls `POST /api/verify/auth`. These SDK endpoints are implemented here.

---

## Build Issues Encountered & Resolved

These are real issues encountered during the build and their solutions:

### Issue 1: Go toolchain auto-download failure
```
go: downloading go1.23 (linux/amd64)
go: download go1.23 for linux/amd64: toolchain not available
```
**Root cause:** `go.mod` had `go 1.23` but system had Go 1.22.2.
**Fix:** `export GOTOOLCHAIN=local && go mod edit -go=1.22.2 && go mod edit -toolchain=none`

### Issue 2: gorilla/sessions v1.4.0 requires Go 1.23+
```
go: github.com/gorilla/sessions@v1.4.0: module requires go >= 1.23
```
**Root cause:** Latest gorilla/sessions bumped its Go requirement. Downgrading to v1.3.0 caused conflicts with `go-ethereum`.
**Fix:** Replaced gorilla/sessions entirely with a standard library implementation. Removed from `go.mod` via `go mod edit -droprequire=github.com/gorilla/sessions`.

### Issue 3: Windows line endings on test script
```
-bash: ./test_veriqid_server.sh: cannot execute: required file not found
```
**Root cause:** Files created on Windows have CRLF line endings. The `#!/bin/bash` shebang becomes `#!/bin/bash\r` which bash can't find.
**Fix:** `sed -i 's/\r$//' test_veriqid_server.sh`

### Issue 4: Empty shell variables in curl commands
```
"message": "invalid character '}' looking for beginning of value"
```
**Root cause:** Shell variables (`$PROOF_HEX`, etc.) were empty because they were set in a different terminal or not set at all.
**Fix:** Run the full flow (get challenge → generate proof → extract values → verify) in the **same terminal session**.

---

## Next Steps: Phase 5 — Browser Extension

With Phase 4 complete, you have a fully functional Veriqid-branded platform server with persistent storage, sessions, and both HTML and JSON API interfaces. Phase 5 creates the Chrome extension (Manifest V3) that automates the proof generation process — the extension detects Veriqid signup/login forms, communicates with the bridge API, and auto-fills the proof fields so the user doesn't need to copy-paste anything.
