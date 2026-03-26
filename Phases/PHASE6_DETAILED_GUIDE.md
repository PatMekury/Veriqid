# Veriqid Phase 6: Parent Dashboard & Onboarding — Complete Detailed Guide

**Time estimate: ~5 hours**
**Goal: Build the Parent Portal — a web application where parents create an account (email/password or phone/code), verify their child through a remote notary or in-person verifier, manage child identities, monitor platform registrations via contract events, and revoke identities with one click. The server manages a custodial Ethereum wallet behind the scenes so parents never interact with blockchain tooling. Uses the Veriqid design system from Phase 5.**

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
>      -port 8080 \
>      -master-secret "my-hackathon-secret"
>    ```
> 5. **Re-create identities** — old key files exist on disk but are not registered on the new contract. Either create fresh keys via the bridge API or re-run the test script.
>
> You will also need to use one of the Ganache private keys (without the `0x` prefix) for any operations that send Ethereum transactions (like creating identities or authorizing verifiers).

---

## BEFORE YOU START: Understanding What You're Building

### The Problem with the Current Architecture

In Phases 1–5, the system works — but only for technical users. Here's what the current flow looks like:

1. A developer runs a CLI command to create an identity (generates msk, registers mpk on-chain)
2. The developer imports a Ganache private key to sign Ethereum transactions
3. The developer knows the contract address, understands ring signatures, and manages key files manually
4. There is **no parent-facing interface at all**

This is unusable for real parents. A parent should never see the words "Ganache," "private key," "contract address," or "ring signature." Phase 6 creates the **Parent Portal** — the first thing a real parent would actually interact with.

### What Phase 6 Builds

```
┌─────────────────────────────────────────────────────────────────┐
│  PARENT PORTAL (Browser — parent.veriqid.com)                   │
│                                                                  │
│  1. ONBOARDING                                                   │
│     ┌─────────────┐  ┌──────────────┐  ┌───────────────┐       │
│     │ Create      │→ │ Add Child    │→ │ Verify Child  │       │
│     │ Account     │  │ (name, age   │  │ (remote notary│       │
│     │ (email+pw   │  │  bracket)    │  │  or in-person)│       │
│     │  or phone)  │  │              │  │               │       │
│     └─────────────┘  └──────────────┘  └───────┬───────┘       │
│                                                 │                │
│  2. DASHBOARD (after verification)              │                │
│     ┌─────────────┐  ┌──────────────┐  ┌───────▼───────┐       │
│     │ Child       │  │ Activity     │  │ Revoke        │       │
│     │ Identity    │  │ Log          │  │ Identity      │       │
│     │ Cards       │  │ (events)     │  │ (one-click)   │       │
│     └─────────────┘  └──────────────┘  └───────────────┘       │
│                                                                  │
└──────────────────────────────┬───────────────────────────────────┘
                               │
          Parent never sees    │  Server handles all
          anything below       │  blockchain interaction
          this line            │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  VERIQID SERVER (Go backend — cmd/veriqid-server)               │
│                                                                  │
│  ┌────────────────┐  ┌──────────────────┐  ┌────────────────┐  │
│  │ Parent Auth    │  │ Custodial Wallet  │  │ Contract       │  │
│  │ (SQLite:       │  │ (server-managed   │  │ Interface      │  │
│  │  email/pw hash │  │  Ethereum keypair │  │ (ethclient +   │  │
│  │  phone/code    │  │  per parent)      │  │  Veriqid.go    │  │
│  │  sessions)     │  │                   │  │  bindings)     │  │
│  └───────┬────────┘  └────────┬──────────┘  └───────┬────────┘  │
│          │                    │                      │           │
│          └────────────────────┼──────────────────────┘           │
│                               │                                  │
└───────────────────────────────┼──────────────────────────────────┘
                                │
                                │ JSON-RPC (localhost:7545)
                                ▼
                   ┌──────────────────────────┐
                   │  Veriqid.sol Contract    │
                   │  (on Ganache)            │
                   └──────────────────────────┘
```

### The Parent Experience (What They Actually See)

**First visit (onboarding — ~3 minutes):**
1. Parent goes to `localhost:8080/parent`
2. Sees "Create your Veriqid account" — enters email + password (or phone + receives SMS code)
3. Sees "Add your child" — enters child's display name (e.g. "Alex") and selects age bracket (Under 13, Teen, Adult)
4. Sees "Verify your child" — chooses remote notarization (video call) or in-person verifier (gets a QR code)
5. After verification completes → "All set! Alex is verified." → redirected to dashboard

**Return visits (dashboard):**
1. Parent goes to `localhost:8080/parent` → logs in with email/password or phone/code
2. Sees dashboard: child identity cards, status, platform activity log
3. Can revoke an identity with one click + password confirmation

**What happens behind the scenes (parent never sees this):**
- Account creation → server generates a custodial Ethereum key pair, encrypts the private key with a key derived from the parent's password, stores it in SQLite
- Add child → server generates the child's msk, derives mpk, holds it pending
- Verification → verifier (remote notary or in-person) confirms → server uses its deployer key (or the verifier's authorized key) to call `addID(mpk, ageBracket)` on the contract → mpk goes on-chain
- Dashboard loads → server queries contract with `getIDSize()`, `getOwner()`, `getState()`, `getAgeBracket()`, filters by the parent's custodial address
- Revoke → server decrypts the custodial private key, signs `revokeID(index)` transaction, submits to Ganache

### Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| **Parent auth** | Email + bcrypt password hash, OR phone + OTP code | Parents already use these patterns everywhere. Zero learning curve |
| **Blockchain interaction** | Server-side custodial wallet | Parents must never touch MetaMask, private keys, or gas fees |
| **Sessions** | HTTP-only secure cookies (extends Phase 4 session system) | Same pattern from Phase 4, proven to work |
| **Storage** | SQLite (extends Phase 4 database) | Already in use, just add parent tables |
| **Child key management** | msk generated server-side, encrypted at rest | Server acts as secure custodian; msk is transferred to extension/app when child sets up device |
| **Verification** | Remote notary or in-person verifier (external process) | Real-world trust anchors; for hackathon, simulated with an "approve" endpoint |
| **Design** | `base.css` from Phase 5 | Consistent Veriqid branding |

---

## PREREQUISITES

Before starting Phase 6, you must have Phases 1–5 fully complete:

```
[x] libsecp256k1 built and installed with --enable-module-ringcip
[x] bridge/bridge.go compiles (go build -o bridge-server ./cmd/bridge/)
[x] Veriqid.sol deployed with events, verifier registry, age brackets (Phase 3)
[x] cmd/veriqid-server running with SQLite, sessions, templates (Phase 4)
[x] Browser extension functional (Phase 5)
[x] static/base.css contains full Veriqid design system (Phase 5)
[x] Ganache running on port 7545
```

You'll also need:
- `golang.org/x/crypto/bcrypt` — for password hashing (go get it in Step 3)
- `crypto/ecdsa` + `crypto/elliptic` — standard library, for custodial wallet key generation
- The Phase 4 SQLite database pattern to extend

---

## STEP 1: Understand the Database Schema Extension

**Time: ~15 minutes (reading, no coding)**

Phase 4 created a SQLite database (`veriqid.db`) with tables for:
- `challenges` — challenge storage with replay protection
- `users` — SPK → username mapping for child platform accounts
- `sessions` — cookie-based session tracking

Phase 6 adds three new tables for the parent system:

### 1.1 New tables

```sql
-- Parent accounts (email/password or phone/code login)
CREATE TABLE IF NOT EXISTS parents (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    email           TEXT UNIQUE,                          -- NULL if phone-based login
    phone           TEXT UNIQUE,                          -- NULL if email-based login
    password_hash   TEXT,                                 -- bcrypt hash (NULL if phone-based)
    eth_address     TEXT NOT NULL UNIQUE,                 -- Custodial wallet public address
    eth_privkey_enc TEXT NOT NULL,                        -- Encrypted custodial private key
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login      DATETIME
);

-- Children linked to parents
CREATE TABLE IF NOT EXISTS children (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id       INTEGER NOT NULL REFERENCES parents(id),
    display_name    TEXT NOT NULL,                        -- "Alex" (parent-chosen, for dashboard only)
    age_bracket     INTEGER NOT NULL DEFAULT 0,           -- 0=unknown, 1=under13, 2=teen, 3=adult
    msk_enc         TEXT,                                 -- Encrypted master secret key (32 bytes)
    mpk_hex         TEXT,                                 -- Master public key (hex, for display)
    contract_index  INTEGER,                              -- Index in Veriqid.sol idList (NULL until verified)
    status          TEXT DEFAULT 'pending',               -- pending | verified | revoked
    verified_at     DATETIME,
    revoked_at      DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Phone OTP codes (for phone-based login)
CREATE TABLE IF NOT EXISTS otp_codes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    phone       TEXT NOT NULL,
    code        TEXT NOT NULL,                            -- 6-digit code
    expires_at  DATETIME NOT NULL,                       -- 5-minute expiry
    used        BOOLEAN DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 1.2 How these relate to the existing tables

```
Phase 4 tables (child/platform side):       Phase 6 tables (parent side):
┌───────────────┐                            ┌────────────────┐
│   challenges  │                            │    parents     │
│  (replay      │                            │  (email/pw or  │
│   protection) │                            │   phone/code)  │
└───────────────┘                            └───────┬────────┘
┌───────────────┐                                    │
│     users     │                            ┌───────▼────────┐
│  (spk→name   │                            │   children     │
│   mapping)    │                            │  (msk, mpk,    │
└───────────────┘                            │   contract idx)│
┌───────────────┐                            └────────────────┘
│   sessions    │                            ┌────────────────┐
│  (cookies)    │                            │   otp_codes    │
└───────────────┘                            │  (phone login) │
                                             └────────────────┘
```

The parent tables are completely separate from the platform-side tables. The `children.contract_index` is the bridge — it references the same index used in `getIDs(index)`, `getState(index)`, and `revokeID(index)` on the contract.

### 1.3 Why encrypt the msk and custodial private key?

If the SQLite database is compromised, the attacker gets:
- **Without encryption:** Every child's master secret key (can impersonate children on all platforms) + every parent's custodial private key (can revoke identities)
- **With encryption:** Useless ciphertext. The keys are encrypted using a derivation of the parent's password (via PBKDF2 or scrypt). Without the password, the keys are unrecoverable.

For the hackathon demo, a simpler approach is acceptable: encrypt using AES-256-GCM with a server-side master key stored in an environment variable. Not as strong as per-parent password-derived encryption, but dramatically simpler to implement and good enough for a demo.

---

## STEP 2: Create the File Structure

**Time: ~5 minutes**

### 2.1 Navigate to the Veriqid directory

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
```

### 2.2 Create new directories

```bash
mkdir -p internal/parent
mkdir -p dashboard
```

### 2.3 Updated directory structure

```
Veriqid/
├── bridge/bridge.go                    ← Bridge API (Phase 2)
├── cmd/
│   ├── bridge/main.go                  ← Bridge entry point (Phase 2)
│   ├── client/main.go                  ← CLI tool (Phase 1)
│   └── veriqid-server/main.go          ← Veriqid server (Phase 4 — extended in Phase 6)
├── contracts/
│   ├── U2SSO.sol                       ← Original contract (Phase 1)
│   └── Veriqid.sol                     ← Enhanced contract (Phase 3)
├── dashboard/
│   ├── index.html                      ← Parent portal: onboarding + dashboard (NEW)
│   ├── dashboard.js                    ← Client-side logic (NEW)
│   └── dashboard.css                   ← Dashboard-specific styles (NEW)
├── extension/                          ← Browser extension (Phase 5)
├── internal/
│   ├── parent/
│   │   ├── auth.go                     ← Parent account creation + login (NEW)
│   │   ├── wallet.go                   ← Custodial Ethereum wallet (NEW)
│   │   └── verification.go            ← Verifier integration (NEW)
│   ├── session/
│   │   ├── session.go                  ← Child session management (Phase 4)
│   │   └── parent_session.go           ← Parent session management (NEW — ParentManager)
│   └── store/
│       ├── sqlite.go                   ← SQLite storage (Phase 4 — extended with parent tables)
│       └── parent_store.go             ← Parent/child/OTP store methods (NEW)
├── pkg/u2sso/                          ← CGO crypto wrapper + contract bindings
├── static/
│   └── base.css                        ← Veriqid design system (Phase 5)
├── templates/                          ← Platform HTML templates (Phase 4/5)
└── ...
```

---

## STEP 3: Install New Dependencies

**Time: ~5 minutes**

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

# bcrypt for password hashing
go get golang.org/x/crypto/bcrypt

# (crypto/ecdsa, crypto/elliptic, crypto/aes, crypto/cipher are all standard library — no install needed)
```

---

## STEP 4: Extend the SQLite Store

**Time: ~30 minutes**

### 4.1 What to add to `internal/store/sqlite.go`

The Phase 4 store has `NewStore()`, challenge methods, and user methods. We're adding parent, child, and OTP methods to the same store.

### 4.2 Add the new tables to the schema initialization

Find the `NewStore()` function in `internal/store/sqlite.go` and add the new `CREATE TABLE` statements after the existing ones:

```go
// Add to the schema initialization in NewStore(), after existing CREATE TABLE statements:

_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS parents (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        email           TEXT UNIQUE,
        phone           TEXT UNIQUE,
        password_hash   TEXT,
        eth_address     TEXT NOT NULL UNIQUE,
        eth_privkey_enc TEXT NOT NULL,
        created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
        last_login      DATETIME
    );

    CREATE TABLE IF NOT EXISTS children (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        parent_id       INTEGER NOT NULL REFERENCES parents(id),
        display_name    TEXT NOT NULL,
        age_bracket     INTEGER NOT NULL DEFAULT 0,
        msk_enc         TEXT,
        mpk_hex         TEXT,
        contract_index  INTEGER,
        status          TEXT DEFAULT 'pending',
        verified_at     DATETIME,
        revoked_at      DATETIME,
        created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS otp_codes (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        phone       TEXT NOT NULL,
        code        TEXT NOT NULL,
        expires_at  DATETIME NOT NULL,
        used        BOOLEAN DEFAULT 0,
        created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
    );
`)
if err != nil {
    return nil, fmt.Errorf("create parent tables: %w", err)
}
```

### 4.3 Add parent store methods

Add the following methods to the `Store` struct (same file, or a new file `internal/store/parent_store.go` if you prefer to keep it separate):

```go
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
    if err != nil {
        return nil, err
    }
    return p, nil
}

// UpdateParentLastLogin updates the parent's last login timestamp.
// NOTE: Named UpdateParentLastLogin (not UpdateLastLogin) to avoid collision
// with the existing Phase 4 UpdateLastLogin method which takes spkHex for platform users.
func (s *Store) UpdateParentLastLogin(parentID int64) error {
    _, err := s.db.Exec(
        `UPDATE parents SET last_login = CURRENT_TIMESTAMP WHERE id = ?`, parentID,
    )
    return err
}

// Parent represents a parent account row.
type Parent struct {
    ID             int64
    Email          *string // nullable
    Phone          *string // nullable
    PasswordHash   *string // nullable (phone-based accounts have no password)
    EthAddress     string
    EthPrivkeyEnc  string
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
    result, err := s.db.Exec(
        `UPDATE otp_codes SET used = 1
         WHERE phone = ? AND code = ? AND used = 0 AND expires_at > datetime('now')`,
        phone, code,
    )
    if err != nil {
        return false, err
    }
    affected, _ := result.RowsAffected()
    return affected > 0, nil
}
```

---

## STEP 5: Build the Custodial Wallet System

**Time: ~25 minutes**

### 5.1 Understanding the custodial wallet

When a parent creates an account, the server generates an Ethereum key pair on their behalf. This key pair is used as the `msg.sender` when calling `addID()` on the contract, making the parent the on-chain "owner" of their child's identity. The private key is encrypted and stored in SQLite. When the parent clicks "Revoke," the server decrypts the key and signs the `revokeID()` transaction.

The parent never sees an Ethereum address, never manages a private key, never pays gas. The server uses the Ganache deployer account to fund custodial wallets with a small amount of ETH for gas.

### 5.2 Create `internal/parent/wallet.go`

```go
package parent

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "math/big"

    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

// GenerateCustodialWallet creates a new Ethereum key pair for a parent.
// Returns: address (hex string), private key (hex string), error
func GenerateCustodialWallet() (address string, privKeyHex string, err error) {
    // Generate a new secp256k1 private key
    privateKey, err := crypto.GenerateKey()
    if err != nil {
        return "", "", fmt.Errorf("generate key: %w", err)
    }

    // Derive the public address
    addr := crypto.PubkeyToAddress(privateKey.PublicKey)

    // Encode private key to hex
    privBytes := crypto.FromECDSA(privateKey)
    privHex := hex.EncodeToString(privBytes)

    return addr.Hex(), privHex, nil
}

// EncryptPrivateKey encrypts a hex-encoded private key using AES-256-GCM.
// The encryptionKey should be a 32-byte key (e.g., from environment variable or derived from password).
func EncryptPrivateKey(privKeyHex string, encryptionKey []byte) (string, error) {
    plaintext := []byte(privKeyHex)

    block, err := aes.NewCipher(encryptionKey)
    if err != nil {
        return "", fmt.Errorf("create cipher: %w", err)
    }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("create GCM: %w", err)
    }

    nonce := make([]byte, aesGCM.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return "", fmt.Errorf("generate nonce: %w", err)
    }

    ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
    return hex.EncodeToString(ciphertext), nil
}

// DecryptPrivateKey decrypts an AES-256-GCM encrypted private key.
func DecryptPrivateKey(encryptedHex string, encryptionKey []byte) (string, error) {
    ciphertext, err := hex.DecodeString(encryptedHex)
    if err != nil {
        return "", fmt.Errorf("decode hex: %w", err)
    }

    block, err := aes.NewCipher(encryptionKey)
    if err != nil {
        return "", fmt.Errorf("create cipher: %w", err)
    }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("create GCM: %w", err)
    }

    nonceSize := aesGCM.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", fmt.Errorf("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", fmt.Errorf("decrypt: %w", err)
    }

    return string(plaintext), nil
}

// DeriveEncryptionKey derives a 32-byte AES key from a master secret (e.g., env variable).
// For the hackathon, use a server-wide master key.
// For production, derive per-parent from their password via scrypt/PBKDF2.
func DeriveEncryptionKey(masterSecret string) []byte {
    hash := sha256.Sum256([]byte(masterSecret))
    return hash[:]
}

// PrivKeyHexToECDSA converts a hex-encoded private key back to *ecdsa.PrivateKey.
// NOTE: Returns *ecdsa.PrivateKey (not *crypto.PrivateKey) — requires "crypto/ecdsa" import.
// Uses crypto.ToECDSA() from go-ethereum, not crypto.HexToECDSA().
func PrivKeyHexToECDSA(privKeyHex string) (*ecdsa.PrivateKey, error) {
    privBytes, err := hex.DecodeString(privKeyHex)
    if err != nil {
        return nil, fmt.Errorf("decode hex: %w", err)
    }
    return crypto.ToECDSA(privBytes)
}
```

### 5.3 Why `go-ethereum/crypto` instead of raw `crypto/ecdsa`?

The `go-ethereum` package (`github.com/ethereum/go-ethereum`) is already a dependency in the project — it's used by the contract bindings generated in Phase 3 (via `abigen`). Its `crypto` subpackage provides Ethereum-specific helpers like `GenerateKey()` (secp256k1), `PubkeyToAddress()`, `FromECDSA()`, and `ToECDSA()` that correctly produce Ethereum-compatible addresses.

### 5.4 Install go-ethereum if not already present

```bash
# This should already be in go.mod from Phase 3's abigen bindings
go get github.com/ethereum/go-ethereum
```

---

## STEP 6: Build the Parent Auth System

**Time: ~30 minutes**

### 6.1 Create `internal/parent/auth.go`

```go
package parent

import (
    "crypto/rand"
    "fmt"
    "math/big"
    "net/mail"
    "regexp"
    "strings"

    "golang.org/x/crypto/bcrypt"
)

const (
    bcryptCost = 12
    otpLength  = 6
)

// ValidateEmail checks if the email format is valid.
func ValidateEmail(email string) error {
    _, err := mail.ParseAddress(email)
    if err != nil {
        return fmt.Errorf("invalid email format")
    }
    return nil
}

// ValidatePhone checks if the phone number is in a reasonable format.
// For the hackathon, accept digits-only, 10-15 characters.
func ValidatePhone(phone string) error {
    cleaned := regexp.MustCompile(`[^0-9+]`).ReplaceAllString(phone, "")
    if len(cleaned) < 10 || len(cleaned) > 15 {
        return fmt.Errorf("phone number must be 10-15 digits")
    }
    return nil
}

// HashPassword creates a bcrypt hash from a plaintext password.
func HashPassword(password string) (string, error) {
    if len(password) < 8 {
        return "", fmt.Errorf("password must be at least 8 characters")
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
    if err != nil {
        return "", fmt.Errorf("hash password: %w", err)
    }
    return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// GenerateOTP creates a cryptographically random 6-digit code.
func GenerateOTP() (string, error) {
    max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(otpLength)), nil)
    n, err := rand.Int(rand.Reader, max)
    if err != nil {
        return "", fmt.Errorf("generate OTP: %w", err)
    }
    return fmt.Sprintf("%06d", n.Int64()), nil
}

// ValidatePassword checks password strength requirements.
func ValidatePassword(password string) error {
    if len(password) < 8 {
        return fmt.Errorf("password must be at least 8 characters")
    }
    if len(password) > 128 {
        return fmt.Errorf("password must be at most 128 characters")
    }
    return nil
}

// NormalizeEmail lowercases and trims whitespace from an email.
func NormalizeEmail(email string) string {
    return strings.ToLower(strings.TrimSpace(email))
}

// NormalizePhone strips non-digit characters except leading +.
func NormalizePhone(phone string) string {
    cleaned := strings.TrimSpace(phone)
    if strings.HasPrefix(cleaned, "+") {
        return "+" + regexp.MustCompile(`[^0-9]`).ReplaceAllString(cleaned[1:], "")
    }
    return regexp.MustCompile(`[^0-9]`).ReplaceAllString(cleaned, "")
}
```

### 6.2 About the OTP (phone login) flow

For the hackathon demo, you have two options for actually sending SMS codes:

**Option A: Console logging (recommended for hackathon).** The server generates the OTP and prints it to the terminal: `[OTP] Code for +1234567890: 482913`. The parent reads it from the demo terminal. This avoids needing an SMS API.

**Option B: Twilio integration.** If you want real SMS, sign up for a free Twilio trial account and use their Go SDK. But this adds complexity — not worth it for a hackathon demo.

The code supports both — the OTP generation and verification logic is the same regardless of delivery method.

---

## STEP 7: Build the API Endpoints

**Time: ~45 minutes**

### 7.1 New endpoints to add to `cmd/veriqid-server/main.go`

The parent portal needs these API routes:

```
POST   /api/parent/register      ← Create parent account (email+pw or phone)
POST   /api/parent/login          ← Login (email+pw or phone+code)
POST   /api/parent/logout         ← Destroy session
POST   /api/parent/send-otp       ← Send OTP to phone (for phone-based login)

POST   /api/parent/child/add      ← Add a child (name, age bracket)
GET    /api/parent/children       ← List parent's children with status
POST   /api/parent/child/revoke   ← Revoke a child's identity

POST   /api/parent/verify/approve ← (HACKATHON ONLY) Simulate verifier approval

GET    /api/parent/events         ← Get contract events for parent's identities
GET    /api/parent/me             ← Get current parent's account info

GET    /parent                    ← Serve the parent dashboard HTML
GET    /dashboard/*               ← Serve dashboard static files (JS, CSS)
```

### 7.2 Registration endpoint

```go
// POST /api/parent/register
// Body: { "email": "parent@example.com", "password": "securepass123" }
//   OR: { "phone": "+11234567890" }
func (s *Server) handleParentRegister(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
        Phone    string `json:"phone"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }

    // Generate custodial wallet for this parent
    ethAddr, privKeyHex, err := parent.GenerateCustodialWallet()
    if err != nil {
        http.Error(w, `{"error":"failed to generate wallet"}`, http.StatusInternalServerError)
        return
    }

    // Encrypt the private key
    encKey := parent.DeriveEncryptionKey(s.masterSecret)
    privKeyEnc, err := parent.EncryptPrivateKey(privKeyHex, encKey)
    if err != nil {
        http.Error(w, `{"error":"failed to encrypt wallet"}`, http.StatusInternalServerError)
        return
    }

    var parentID int64

    if req.Email != "" {
        // Email + password registration
        email := parent.NormalizeEmail(req.Email)
        if err := parent.ValidateEmail(email); err != nil {
            http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
            return
        }
        if err := parent.ValidatePassword(req.Password); err != nil {
            http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
            return
        }

        passwordHash, err := parent.HashPassword(req.Password)
        if err != nil {
            http.Error(w, `{"error":"failed to hash password"}`, http.StatusInternalServerError)
            return
        }

        parentID, err = s.store.CreateParentByEmail(email, passwordHash, ethAddr, privKeyEnc)
        if err != nil {
            // Likely duplicate email
            http.Error(w, `{"error":"email already registered"}`, http.StatusConflict)
            return
        }

    } else if req.Phone != "" {
        // Phone registration — create account, then send OTP to verify
        phone := parent.NormalizePhone(req.Phone)
        if err := parent.ValidatePhone(phone); err != nil {
            http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
            return
        }

        parentID, err = s.store.CreateParentByPhone(phone, ethAddr, privKeyEnc)
        if err != nil {
            http.Error(w, `{"error":"phone already registered"}`, http.StatusConflict)
            return
        }

        // Generate and "send" OTP
        code, _ := parent.GenerateOTP()
        s.store.StoreOTP(phone, code)
        fmt.Printf("[OTP] Code for %s: %s\n", phone, code)  // Console logging for demo

    } else {
        http.Error(w, `{"error":"provide email+password or phone"}`, http.StatusBadRequest)
        return
    }

    isPhone := req.Phone != ""

    if !isPhone {
        // Email registration: auto-login immediately (password already verified)
        s.ParentSessions.Login(w, parentID)
    }
    // Phone registration: do NOT auto-login here.
    // The parent must verify the OTP via /api/parent/login first.
    // This prevents the session cookie from being set before phone ownership is confirmed.

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":      true,
        "parent_id":    parentID,
        "method":       map[bool]string{true: "email", false: "phone"}[req.Email != ""],
        "otp_required": isPhone,
    })
}
```

### 7.3 Login endpoint

```go
// POST /api/parent/login
// Body: { "email": "parent@example.com", "password": "securepass123" }
//   OR: { "phone": "+11234567890", "code": "482913" }
func (s *Server) handleParentLogin(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
        Phone    string `json:"phone"`
        Code     string `json:"code"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }

    var p *store.Parent
    var err error

    if req.Email != "" {
        // Email + password login
        email := parent.NormalizeEmail(req.Email)
        p, err = s.store.GetParentByEmail(email)
        if err != nil {
            http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
            return
        }
        if p.PasswordHash == nil || !parent.CheckPassword(req.Password, *p.PasswordHash) {
            http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
            return
        }

    } else if req.Phone != "" && req.Code != "" {
        // Phone + OTP login
        phone := parent.NormalizePhone(req.Phone)
        valid, err := s.store.VerifyOTP(phone, req.Code)
        if err != nil || !valid {
            http.Error(w, `{"error":"invalid or expired code"}`, http.StatusUnauthorized)
            return
        }
        p, err = s.store.GetParentByPhone(phone)
        if err != nil {
            http.Error(w, `{"error":"account not found"}`, http.StatusUnauthorized)
            return
        }

    } else {
        http.Error(w, `{"error":"provide email+password or phone+code"}`, http.StatusBadRequest)
        return
    }

    // Update last login
    s.store.UpdateLastLogin(p.ID)

    // Create session
    sessionID := s.sessions.Create(p.ID)
    http.SetCookie(w, &http.Cookie{
        Name:     "veriqid_parent_session",
        Value:    sessionID,
        Path:     "/",
        HttpOnly: true,
        MaxAge:   3600 * 24,
    })

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":   true,
        "parent_id": p.ID,
    })
}
```

### 7.4 Add child endpoint

```go
// POST /api/parent/child/add
// Body: { "display_name": "Alex", "age_bracket": 1 }
// Requires parent session
func (s *Server) handleAddChild(w http.ResponseWriter, r *http.Request) {
    parentID, ok := s.getParentFromSession(r)
    if !ok {
        http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
        return
    }

    var req struct {
        DisplayName string `json:"display_name"`
        AgeBracket  int    `json:"age_bracket"` // 0=unknown, 1=under13, 2=teen, 3=adult
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }

    if req.DisplayName == "" {
        http.Error(w, `{"error":"display_name is required"}`, http.StatusBadRequest)
        return
    }
    if req.AgeBracket < 0 || req.AgeBracket > 3 {
        http.Error(w, `{"error":"age_bracket must be 0-3"}`, http.StatusBadRequest)
        return
    }

    // Generate the child's master secret key (msk) via the bridge API
    // The bridge's POST /api/identity/create generates msk + mpk and registers on-chain
    // But for Phase 6, we generate msk locally and hold it PENDING until verification
    msk := make([]byte, 32)
    if _, err := rand.Read(msk); err != nil {
        http.Error(w, `{"error":"failed to generate key"}`, http.StatusInternalServerError)
        return
    }
    mskHex := hex.EncodeToString(msk)

    // Derive mpk from msk using the CGO library (CreateID).
    // NOTE: The function is u2sso.CreateID(), NOT u2sso.DeriveMPK() — DeriveMPK does not exist.
    // CreateID() takes msk bytes and returns mpk bytes via libsecp256k1's secp256k1_boquila_gen_mpk.
    mpkBytes := u2sso.CreateID(msk)
    var mpkHex string
    if len(mpkBytes) == 0 {
        // CGO not available — use placeholder for hackathon demo
        log.Printf("WARNING: CreateID returned empty mpk — using placeholder")
        mpkHex = mskHex
    } else {
        mpkHex = hex.EncodeToString(mpkBytes)
    }

    // Encrypt msk for storage
    encKey := parent.DeriveEncryptionKey(s.masterSecret)
    mskEnc, err := parent.EncryptPrivateKey(mskHex, encKey)
    if err != nil {
        http.Error(w, `{"error":"failed to encrypt key"}`, http.StatusInternalServerError)
        return
    }

    // Store child in database (status = 'pending' until verified)
    childID, err := s.store.AddChild(parentID, req.DisplayName, req.AgeBracket, mskEnc, mpkHex)
    if err != nil {
        http.Error(w, `{"error":"failed to add child"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "child_id": childID,
        "status":   "pending",
        "message":  "Child added. Complete verification to activate the identity.",
    })
}
```

### 7.5 Verification approval endpoint (hackathon simulation)

In a production system, verification would involve a real remote notary or in-person verifier who independently calls the contract. For the hackathon demo, we simulate this with an endpoint that the "verifier" clicks to approve:

```go
// POST /api/parent/verify/approve
// Body: { "child_id": 1 }
// In production: this would be called by the verifier's system after remote notarization.
// For hackathon: called directly to simulate verifier approval.
func (s *Server) handleVerifyApprove(w http.ResponseWriter, r *http.Request) {
    parentID, ok := s.getParentFromSession(r)
    if !ok {
        http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
        return
    }

    var req struct {
        ChildID int64 `json:"child_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }

    // Get the child record
    children, err := s.store.GetChildrenByParent(parentID)
    if err != nil {
        http.Error(w, `{"error":"failed to load children"}`, http.StatusInternalServerError)
        return
    }

    var child *store.Child
    for _, c := range children {
        if c.ID == req.ChildID {
            child = &c
            break
        }
    }

    if child == nil {
        http.Error(w, `{"error":"child not found"}`, http.StatusNotFound)
        return
    }

    if child.Status != "pending" {
        http.Error(w, `{"error":"child already verified or revoked"}`, http.StatusBadRequest)
        return
    }

    // Register the mpk on-chain via the contract
    // Uses the deployer/admin key (since admin is an authorized verifier from Phase 3)
    contractIndex, err := s.registerOnChain(child.MpkHex, child.AgeBracket)
    if err != nil {
        http.Error(w, fmt.Sprintf(`{"error":"on-chain registration failed: %s"}`, err.Error()),
            http.StatusInternalServerError)
        return
    }

    // Update child record
    if err := s.store.MarkChildVerified(req.ChildID, contractIndex); err != nil {
        http.Error(w, `{"error":"failed to update child status"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":        true,
        "child_id":       req.ChildID,
        "contract_index": contractIndex,
        "status":         "verified",
        "message":        "Identity verified and registered on-chain.",
    })
}
```

### 7.6 Revocation endpoint

```go
// POST /api/parent/child/revoke
// Body: { "child_id": 1, "password": "securepass123" }
// Requires re-authentication (password or OTP) for safety
func (s *Server) handleRevokeChild(w http.ResponseWriter, r *http.Request) {
    parentID, ok := s.getParentFromSession(r)
    if !ok {
        http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
        return
    }

    var req struct {
        ChildID  int64  `json:"child_id"`
        Password string `json:"password"` // Re-authenticate for destructive action
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
        return
    }

    // Re-authenticate the parent (extra safety for destructive action)
    parentRecord, err := s.store.GetParentByID(parentID)
    if err != nil {
        http.Error(w, `{"error":"parent not found"}`, http.StatusInternalServerError)
        return
    }
    if parentRecord.PasswordHash != nil && !parent.CheckPassword(req.Password, *parentRecord.PasswordHash) {
        http.Error(w, `{"error":"incorrect password"}`, http.StatusUnauthorized)
        return
    }

    // Find the child
    children, err := s.store.GetChildrenByParent(parentID)
    if err != nil {
        http.Error(w, `{"error":"failed to load children"}`, http.StatusInternalServerError)
        return
    }

    var child *store.Child
    for _, c := range children {
        if c.ID == req.ChildID {
            child = &c
            break
        }
    }

    if child == nil {
        http.Error(w, `{"error":"child not found"}`, http.StatusNotFound)
        return
    }

    if child.Status != "verified" || child.ContractIndex == nil {
        http.Error(w, `{"error":"child is not in a revocable state"}`, http.StatusBadRequest)
        return
    }

    // Call revokeID() on the contract using the admin key
    // (The admin can revoke any identity per Phase 3's Veriqid.sol)
    err = s.revokeOnChain(*child.ContractIndex)
    if err != nil {
        http.Error(w, fmt.Sprintf(`{"error":"revocation failed: %s"}`, err.Error()),
            http.StatusInternalServerError)
        return
    }

    // Update local database
    if err := s.store.MarkChildRevoked(req.ChildID); err != nil {
        http.Error(w, `{"error":"failed to update status"}`, http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "child_id": req.ChildID,
        "status":   "revoked",
        "message":  "Identity has been permanently revoked across all platforms.",
    })
}
```

### 7.7 Children listing endpoint

```go
// GET /api/parent/children
// Returns all children for the logged-in parent with current on-chain status
func (s *Server) handleGetChildren(w http.ResponseWriter, r *http.Request) {
    parentID, ok := s.getParentFromSession(r)
    if !ok {
        http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
        return
    }

    children, err := s.store.GetChildrenByParent(parentID)
    if err != nil {
        http.Error(w, `{"error":"failed to load children"}`, http.StatusInternalServerError)
        return
    }

    // Enrich with live on-chain status (in case it changed outside the dashboard)
    type ChildResponse struct {
        ID            int64   `json:"id"`
        DisplayName   string  `json:"display_name"`
        AgeBracket    int     `json:"age_bracket"`
        AgeBracketStr string  `json:"age_bracket_label"`
        MpkHex        string  `json:"mpk_hex"`
        ContractIndex *int    `json:"contract_index"`
        Status        string  `json:"status"`
        VerifiedAt    *string `json:"verified_at"`
        RevokedAt     *string `json:"revoked_at"`
        CreatedAt     string  `json:"created_at"`
    }

    ageBracketLabels := map[int]string{0: "Unknown", 1: "Under 13", 2: "Teen (13–17)", 3: "Adult (18+)"}

    var response []ChildResponse
    for _, c := range children {
        cr := ChildResponse{
            ID:            c.ID,
            DisplayName:   c.DisplayName,
            AgeBracket:    c.AgeBracket,
            AgeBracketStr: ageBracketLabels[c.AgeBracket],
            ContractIndex: c.ContractIndex,
            Status:        c.Status,
            VerifiedAt:    c.VerifiedAt,
            RevokedAt:     c.RevokedAt,
            CreatedAt:     c.CreatedAt,
        }
        if c.MpkHex != nil {
            cr.MpkHex = *c.MpkHex
        }

        // Cross-check on-chain status for verified children
        if c.ContractIndex != nil && c.Status == "verified" {
            onChainActive, err := s.checkOnChainStatus(*c.ContractIndex)
            if err == nil && !onChainActive {
                // Revoked on-chain but not in our DB — sync it
                s.store.MarkChildRevoked(c.ID)
                cr.Status = "revoked"
            }
        }

        response = append(response, cr)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "children": response,
    })
}
```

### 7.8 Contract events endpoint

```go
// GET /api/parent/events
// Returns IDRegistered and IDRevoked events for the parent's children
func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
    parentID, ok := s.getParentFromSession(r)
    if !ok {
        http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
        return
    }

    children, err := s.store.GetChildrenByParent(parentID)
    if err != nil {
        http.Error(w, `{"error":"failed to load children"}`, http.StatusInternalServerError)
        return
    }

    // Collect contract indices for the parent's children
    var indices []int
    childNames := make(map[int]string) // contract_index → display name
    for _, c := range children {
        if c.ContractIndex != nil {
            indices = append(indices, *c.ContractIndex)
            childNames[*c.ContractIndex] = c.DisplayName
        }
    }

    // Query contract events for those indices
    events, err := s.getContractEvents(indices)
    if err != nil {
        http.Error(w, `{"error":"failed to load events"}`, http.StatusInternalServerError)
        return
    }

    // Enrich events with child display names
    for i, evt := range events {
        if name, ok := childNames[evt.ContractIndex]; ok {
            events[i].ChildName = name
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "events":  events,
    })
}
```

### 7.9 Helper: get parent from session

```go
// getParentFromSession extracts the parent ID from the session cookie.
func (s *Server) getParentFromSession(r *http.Request) (int64, bool) {
    cookie, err := r.Cookie("veriqid_parent_session")
    if err != nil {
        return 0, false
    }
    parentID, ok := s.sessions.Get(cookie.Value)
    if !ok {
        return 0, false
    }
    return parentID, true
}
```

### 7.10 Register routes in main()

In `cmd/veriqid-server/main.go`, add the new routes:

```go
// Parent Portal API
mux.HandleFunc("/api/parent/register", s.handleParentRegister)
mux.HandleFunc("/api/parent/login", s.handleParentLogin)
mux.HandleFunc("/api/parent/logout", s.handleParentLogout)
mux.HandleFunc("/api/parent/send-otp", s.handleSendOTP)
mux.HandleFunc("/api/parent/me", s.handleGetParentInfo)
mux.HandleFunc("/api/parent/children", s.handleGetChildren)
mux.HandleFunc("/api/parent/child/add", s.handleAddChild)
mux.HandleFunc("/api/parent/child/revoke", s.handleRevokeChild)
mux.HandleFunc("/api/parent/verify/approve", s.handleVerifyApprove)
mux.HandleFunc("/api/parent/events", s.handleGetEvents)

// Parent dashboard static files
dashboardDir := http.Dir("./dashboard")
mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", http.FileServer(dashboardDir)))
mux.HandleFunc("/parent", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "./dashboard/index.html")
})

// Add master secret for encryption (from env or flag)
// -master-secret flag or VERIQID_MASTER_SECRET env variable
masterSecret := flag.String("master-secret", "hackathon-demo-key-change-in-production", "Encryption master secret")
```

---

## STEP 8: Build the Dashboard Frontend

**Time: ~40 minutes**

### 8.1 Understanding the frontend architecture

The dashboard is a single-page app (SPA) using vanilla JavaScript. No framework, no build step. It talks to the API endpoints from Step 7 via `fetch()`.

```
dashboard/index.html
├── Screen: Login/Register (shown when not logged in)
│   ├── Tab: Email + Password
│   └── Tab: Phone + Code
├── Screen: Onboarding (shown after first login if no children)
│   ├── Add Child form (name + age bracket)
│   └── Verification prompt
└── Screen: Dashboard (shown when logged in with verified children)
    ├── Summary stats (total, active, revoked)
    ├── Child identity cards (status, age, revoke button)
    └── Activity log (contract events)
```

### 8.2 Create `dashboard/index.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Veriqid — Parent Portal</title>

    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">

    <link rel="stylesheet" href="/static/base.css">
    <link rel="stylesheet" href="/dashboard/dashboard.css">
</head>
<body>

    <!-- ─── Navigation ─────────────────────────────────────── -->
    <nav class="vq-nav">
        <div class="vq-nav-inner">
            <a href="/" class="vq-nav-brand">
                <span class="vq-nav-logo">🛡️</span>
                <span>Veriqid</span>
            </a>
            <div class="vq-nav-links">
                <a href="/" class="vq-nav-link">Home</a>
                <a href="/parent" class="vq-nav-link active">Parent Portal</a>
            </div>
            <div id="user-info" class="user-info" style="display:none;">
                <span id="user-email" class="user-email"></span>
                <button class="vq-btn-link" onclick="logout()">Log out</button>
            </div>
        </div>
    </nav>

    <!-- ═══ SCREEN: AUTH (Login / Register) ═══════════════════ -->
    <section id="auth-screen" class="auth-section">
        <div class="auth-card">
            <div class="auth-icon">🛡️</div>
            <h1>Parent Portal</h1>
            <p>Manage your child's digital identity — safely and privately.</p>

            <!-- Tab switcher: Login / Register -->
            <div class="auth-tabs">
                <button class="auth-tab active" data-tab="login" onclick="switchAuthTab('login')">Log In</button>
                <button class="auth-tab" data-tab="register" onclick="switchAuthTab('register')">Create Account</button>
            </div>

            <!-- Method switcher: Email / Phone -->
            <div class="method-tabs">
                <button class="method-tab active" data-method="email" onclick="switchMethodTab('email')">Email</button>
                <button class="method-tab" data-method="phone" onclick="switchMethodTab('phone')">Phone</button>
            </div>

            <!-- Email form -->
            <form id="form-email" class="auth-form" onsubmit="handleEmailSubmit(event)">
                <div class="form-group">
                    <label for="email">Email address</label>
                    <input type="email" id="email" class="input" placeholder="parent@example.com" required>
                </div>
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" class="input" placeholder="At least 8 characters" required minlength="8">
                </div>
                <button type="submit" id="btn-email-submit" class="vq-btn-primary">
                    <span id="btn-email-text">Log In</span>
                </button>
            </form>

            <!-- Phone form -->
            <form id="form-phone" class="auth-form" style="display:none;" onsubmit="handlePhoneSubmit(event)">
                <div class="form-group">
                    <label for="phone">Phone number</label>
                    <input type="tel" id="phone" class="input" placeholder="+1 (234) 567-8900" required>
                </div>
                <div id="otp-group" class="form-group" style="display:none;">
                    <label for="otp-code">Verification code</label>
                    <input type="text" id="otp-code" class="input" placeholder="6-digit code" maxlength="6" pattern="[0-9]{6}">
                    <span class="help-text">Check your phone for the code (or the server terminal for demo).</span>
                </div>
                <button type="submit" id="btn-phone-submit" class="vq-btn-primary">
                    <span id="btn-phone-text">Send Code</span>
                </button>
            </form>

            <p id="auth-error" class="auth-error" style="display:none;"></p>
        </div>
    </section>

    <!-- ═══ SCREEN: ONBOARDING (Add Child + Verify) ═══════════ -->
    <section id="onboarding-screen" class="onboarding-section" style="display:none;">
        <div class="onboarding-card">

            <!-- Step 1: Add child -->
            <div id="onboard-step1">
                <div class="onboard-icon">👶</div>
                <h1>Add Your Child</h1>
                <p>Enter your child's name and age range. This information is stored locally and never shared with platforms.</p>

                <form onsubmit="handleAddChild(event)">
                    <div class="form-group">
                        <label for="child-name">Child's name or nickname</label>
                        <input type="text" id="child-name" class="input" placeholder="e.g. Alex" required>
                        <span class="help-text">Only visible to you on this dashboard.</span>
                    </div>
                    <div class="form-group">
                        <label for="child-age">Age range</label>
                        <select id="child-age" class="input">
                            <option value="1">Under 13</option>
                            <option value="2">13–17 (Teen)</option>
                            <option value="3">18+ (Adult)</option>
                        </select>
                    </div>
                    <button type="submit" class="vq-btn-primary">Continue</button>
                </form>
            </div>

            <!-- Step 2: Verification -->
            <div id="onboard-step2" style="display:none;">
                <div class="onboard-icon">✅</div>
                <h1>Verify <span id="verify-child-name"></span>'s Identity</h1>
                <p>Choose how you'd like to verify your child's age. This is required to activate their digital identity.</p>

                <div class="verify-options">
                    <div class="verify-option" onclick="selectVerifyMethod('remote')">
                        <div class="verify-option-icon">💻</div>
                        <h3>Remote Notarization</h3>
                        <p>5-minute video call with a certified notary. Show your ID and child's birth certificate on camera.</p>
                        <span class="verify-badge">Recommended</span>
                    </div>
                    <div class="verify-option" onclick="selectVerifyMethod('inperson')">
                        <div class="verify-option-icon">🏥</div>
                        <h3>In-Person Verifier</h3>
                        <p>Visit a participating pediatrician, school, or government office with your documents.</p>
                    </div>
                </div>

                <!-- Hackathon: simulate verification -->
                <div id="verify-demo-section" style="display:none;">
                    <div class="verify-demo-note">
                        <strong>Hackathon Demo Mode:</strong> Click below to simulate verifier approval.
                        In production, this would be completed by a remote notary or in-person verifier.
                    </div>
                    <button id="btn-verify-approve" class="vq-btn-primary" onclick="simulateVerification()">
                        ✓ Simulate Verification Approval
                    </button>
                </div>
            </div>

            <!-- Step 3: Success -->
            <div id="onboard-step3" style="display:none;">
                <div class="onboard-icon">🎉</div>
                <h1>All Set!</h1>
                <p><span id="success-child-name"></span>'s identity is verified and active on the blockchain.</p>
                <p class="success-subtext">
                    Next: install the Veriqid browser extension or mobile app on your child's device
                    to start using verified accounts on platforms.
                </p>
                <button class="vq-btn-primary" onclick="goToDashboard()">Go to Dashboard</button>
            </div>

        </div>
    </section>

    <!-- ═══ SCREEN: DASHBOARD ═══════════════════════════════════ -->
    <main id="dashboard-screen" class="dashboard-main" style="display:none;">

        <!-- Summary Cards -->
        <div class="summary-row">
            <div class="summary-card">
                <div class="summary-label">Children</div>
                <div id="stat-total" class="summary-value">—</div>
                <div class="summary-desc">Registered identities</div>
            </div>
            <div class="summary-card summary-card-active">
                <div class="summary-label">Active</div>
                <div id="stat-active" class="summary-value accent">—</div>
                <div class="summary-desc">Currently verified</div>
            </div>
            <div class="summary-card summary-card-revoked">
                <div class="summary-label">Revoked</div>
                <div id="stat-revoked" class="summary-value">—</div>
                <div class="summary-desc">Deactivated</div>
            </div>
        </div>

        <!-- Add Another Child -->
        <div class="add-child-bar">
            <button class="vq-btn-secondary" onclick="showOnboarding()">+ Add Another Child</button>
        </div>

        <!-- Child Cards -->
        <section class="section-block">
            <div class="section-header">
                <h2>Your Children</h2>
                <button class="vq-btn-secondary btn-sm" onclick="refreshDashboard()">↻ Refresh</button>
            </div>
            <div id="children-grid" class="identity-grid">
                <div class="loading-placeholder">
                    <div class="spinner"></div>
                    <p>Loading...</p>
                </div>
            </div>
        </section>

        <!-- Activity Log -->
        <section class="section-block">
            <div class="section-header">
                <h2>Activity Log</h2>
                <span class="badge" id="event-count">0 events</span>
            </div>
            <div id="activity-log" class="activity-log">
                <div class="loading-placeholder">
                    <div class="spinner"></div>
                    <p>Loading events...</p>
                </div>
            </div>
        </section>

    </main>

    <!-- ═══ REVOKE MODAL ════════════════════════════════════════ -->
    <div id="revoke-modal" class="modal-overlay" style="display:none;">
        <div class="modal-card">
            <div class="modal-icon">⚠️</div>
            <h2>Revoke Identity?</h2>
            <p>This will <strong>permanently deactivate</strong> <span id="revoke-child-name"></span>'s identity.</p>
            <p class="modal-warning">
                Once revoked, your child will lose access to ALL platforms where this identity was used.
                This cannot be undone.
            </p>
            <div class="form-group">
                <label for="revoke-password">Enter your password to confirm</label>
                <input type="password" id="revoke-password" class="input" placeholder="Your account password">
            </div>
            <div class="modal-actions">
                <button class="vq-btn-secondary" onclick="closeRevokeModal()">Cancel</button>
                <button id="btn-confirm-revoke" class="btn-danger" onclick="confirmRevoke()">
                    Revoke Identity
                </button>
            </div>
            <p id="revoke-status" class="revoke-status" style="display:none;"></p>
        </div>
    </div>

    <!-- Footer -->
    <footer class="vq-footer">
        <div class="vq-footer-inner">
            <span>🛡️ Veriqid — Privacy-First Children's Internet Identity</span>
            <span class="vq-footer-sub">Parent Portal v0.6.0</span>
        </div>
    </footer>

    <script src="/dashboard/dashboard.js"></script>

</body>
</html>
```

### 8.3 Create `dashboard/dashboard.js`

```javascript
// ═══════════════════════════════════════════════════════════════
// Veriqid Parent Portal — Client-side JavaScript
// Communicates with /api/parent/* endpoints via fetch()
// No MetaMask, no ethers.js — all blockchain interaction is server-side
// ═══════════════════════════════════════════════════════════════

// ── State ────────────────────────────────────────────────────
let currentTab = 'login';     // 'login' or 'register'
let currentMethod = 'email';  // 'email' or 'phone'
let otpSent = false;
let pendingChildID = null;    // For onboarding flow
let revokeChildID = null;     // For revoke modal

const AGE_LABELS = { 0: 'Unknown', 1: 'Under 13', 2: 'Teen (13–17)', 3: 'Adult (18+)' };
const AGE_COLORS = { 0: '#9ca3af', 1: '#fbbf24', 2: '#60a5fa', 3: '#4ade80' };
const STATUS_CONFIG = {
    pending:  { label: 'Pending', class: 'pending',  icon: '⏳' },
    verified: { label: 'Active',  class: 'active',   icon: '✓' },
    revoked:  { label: 'Revoked', class: 'revoked',  icon: '✗' }
};

// ── Auth Tab Switching ───────────────────────────────────────
function switchAuthTab(tab) {
    currentTab = tab;
    document.querySelectorAll('.auth-tab').forEach(t => t.classList.remove('active'));
    document.querySelector(`.auth-tab[data-tab="${tab}"]`).classList.add('active');
    // Update button text
    document.getElementById('btn-email-text').textContent = tab === 'login' ? 'Log In' : 'Create Account';
    document.getElementById('btn-phone-text').textContent = otpSent ? 'Verify Code' : 'Send Code';
}

function switchMethodTab(method) {
    currentMethod = method;
    document.querySelectorAll('.method-tab').forEach(t => t.classList.remove('active'));
    document.querySelector(`.method-tab[data-method="${method}"]`).classList.add('active');
    document.getElementById('form-email').style.display = method === 'email' ? 'block' : 'none';
    document.getElementById('form-phone').style.display = method === 'phone' ? 'block' : 'none';
}

// ── Email Auth ───────────────────────────────────────────────
async function handleEmailSubmit(e) {
    e.preventDefault();
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const errorEl = document.getElementById('auth-error');
    errorEl.style.display = 'none';

    const endpoint = currentTab === 'login' ? '/api/parent/login' : '/api/parent/register';

    try {
        const res = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });

        const data = await res.json();
        if (!res.ok) {
            errorEl.textContent = data.error || 'Something went wrong.';
            errorEl.style.display = 'block';
            return;
        }

        // Success — check if parent has children
        await onLoginSuccess(email);

    } catch (err) {
        errorEl.textContent = 'Network error. Is the server running?';
        errorEl.style.display = 'block';
    }
}

// ── Phone Auth ───────────────────────────────────────────────
async function handlePhoneSubmit(e) {
    e.preventDefault();
    const phone = document.getElementById('phone').value;
    const errorEl = document.getElementById('auth-error');
    errorEl.style.display = 'none';

    if (!otpSent) {
        // Step 1: Send OTP (for both register and login)
        const endpoint = currentTab === 'login' ? '/api/parent/send-otp' : '/api/parent/register';
        const body = currentTab === 'login' ? { phone } : { phone };

        try {
            const res = await fetch(endpoint, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });

            const data = await res.json();
            if (!res.ok) {
                errorEl.textContent = data.error || 'Failed to send code.';
                errorEl.style.display = 'block';
                return;
            }

            // Show OTP input
            otpSent = true;
            document.getElementById('otp-group').style.display = 'block';
            document.getElementById('btn-phone-text').textContent = 'Verify Code';

        } catch (err) {
            errorEl.textContent = 'Network error.';
            errorEl.style.display = 'block';
        }

    } else {
        // Step 2: Verify OTP
        const code = document.getElementById('otp-code').value;

        try {
            const res = await fetch('/api/parent/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ phone, code })
            });

            const data = await res.json();
            if (!res.ok) {
                errorEl.textContent = data.error || 'Invalid code.';
                errorEl.style.display = 'block';
                return;
            }

            await onLoginSuccess(phone);

        } catch (err) {
            errorEl.textContent = 'Network error.';
            errorEl.style.display = 'block';
        }
    }
}

// ── Post-Login Logic ─────────────────────────────────────────
async function onLoginSuccess(identifier) {
    // Update nav
    document.getElementById('user-info').style.display = 'flex';
    document.getElementById('user-email').textContent = identifier;

    // Check if parent has children
    const res = await fetch('/api/parent/children');
    const data = await res.json();

    if (data.children && data.children.length > 0) {
        showDashboard(data.children);
    } else {
        showOnboarding();
    }
}

// ── Screen Navigation ────────────────────────────────────────
function showOnboarding() {
    document.getElementById('auth-screen').style.display = 'none';
    document.getElementById('onboarding-screen').style.display = 'flex';
    document.getElementById('dashboard-screen').style.display = 'none';
    // Reset to step 1
    document.getElementById('onboard-step1').style.display = 'block';
    document.getElementById('onboard-step2').style.display = 'none';
    document.getElementById('onboard-step3').style.display = 'none';
}

function showDashboard(children) {
    document.getElementById('auth-screen').style.display = 'none';
    document.getElementById('onboarding-screen').style.display = 'none';
    document.getElementById('dashboard-screen').style.display = 'block';
    renderChildren(children);
    loadEvents();
}

function goToDashboard() {
    refreshDashboard();
}

// ── Add Child ────────────────────────────────────────────────
async function handleAddChild(e) {
    e.preventDefault();
    const name = document.getElementById('child-name').value;
    const ageBracket = parseInt(document.getElementById('child-age').value);

    try {
        const res = await fetch('/api/parent/child/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ display_name: name, age_bracket: ageBracket })
        });

        const data = await res.json();
        if (!res.ok) {
            alert(data.error || 'Failed to add child.');
            return;
        }

        pendingChildID = data.child_id;

        // Move to verification step
        document.getElementById('verify-child-name').textContent = name;
        document.getElementById('onboard-step1').style.display = 'none';
        document.getElementById('onboard-step2').style.display = 'block';

    } catch (err) {
        alert('Network error.');
    }
}

// ── Verification ─────────────────────────────────────────────
function selectVerifyMethod(method) {
    // Highlight selected option
    document.querySelectorAll('.verify-option').forEach(o => o.classList.remove('selected'));
    event.currentTarget.classList.add('selected');

    // Show demo simulation button
    document.getElementById('verify-demo-section').style.display = 'block';
}

async function simulateVerification() {
    const btn = document.getElementById('btn-verify-approve');
    btn.disabled = true;
    btn.textContent = 'Verifying...';

    try {
        const res = await fetch('/api/parent/verify/approve', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ child_id: pendingChildID })
        });

        const data = await res.json();
        if (!res.ok) {
            alert(data.error || 'Verification failed.');
            btn.disabled = false;
            btn.textContent = '✓ Simulate Verification Approval';
            return;
        }

        // Show success
        const childName = document.getElementById('verify-child-name').textContent;
        document.getElementById('success-child-name').textContent = childName;
        document.getElementById('onboard-step2').style.display = 'none';
        document.getElementById('onboard-step3').style.display = 'block';

    } catch (err) {
        alert('Network error.');
        btn.disabled = false;
        btn.textContent = '✓ Simulate Verification Approval';
    }
}

// ── Dashboard Rendering ──────────────────────────────────────
async function refreshDashboard() {
    try {
        const res = await fetch('/api/parent/children');
        const data = await res.json();

        if (data.children) {
            showDashboard(data.children);
        }
    } catch (err) {
        console.error('Failed to refresh:', err);
    }
}

function renderChildren(children) {
    const grid = document.getElementById('children-grid');

    // Update summary
    const total = children.length;
    const active = children.filter(c => c.status === 'verified').length;
    const revoked = children.filter(c => c.status === 'revoked').length;
    document.getElementById('stat-total').textContent = total;
    document.getElementById('stat-active').textContent = active;
    document.getElementById('stat-revoked').textContent = revoked;

    if (children.length === 0) {
        grid.innerHTML = '<div class="empty-state"><h3>No children added yet.</h3></div>';
        return;
    }

    grid.innerHTML = children.map(child => {
        const status = STATUS_CONFIG[child.status] || STATUS_CONFIG.pending;
        const mpkShort = child.mpk_hex
            ? `0x${child.mpk_hex.slice(0, 8)}...${child.mpk_hex.slice(-6)}`
            : '—';

        let actionBtn = '';
        if (child.status === 'verified') {
            actionBtn = `<button class="btn-danger" onclick="openRevokeModal(${child.id}, '${child.display_name}')">Revoke Identity</button>`;
        } else if (child.status === 'pending') {
            actionBtn = `<button class="vq-btn-primary btn-sm" onclick="showOnboarding()">Complete Verification</button>`;
        } else {
            actionBtn = `<div class="btn-revoked">✗ Revoked</div>`;
        }

        return `
            <div class="identity-card">
                <div class="identity-card-header">
                    <div class="identity-card-title">
                        <span class="identity-index">${child.display_name}</span>
                    </div>
                    <span class="status-badge ${status.class}">${status.icon} ${status.label}</span>
                </div>
                <div class="identity-card-body">
                    <div class="identity-detail">
                        <span class="identity-detail-label">Age Range</span>
                        <span class="identity-detail-value" style="color:${AGE_COLORS[child.age_bracket]}">${child.age_bracket_label}</span>
                    </div>
                    <div class="identity-detail">
                        <span class="identity-detail-label">Public Key</span>
                        <span class="identity-detail-value mono">${mpkShort}</span>
                    </div>
                    <div class="identity-detail">
                        <span class="identity-detail-label">Verified</span>
                        <span class="identity-detail-value">${child.verified_at || 'Not yet'}</span>
                    </div>
                </div>
                <div class="identity-card-actions">${actionBtn}</div>
            </div>
        `;
    }).join('');
}

// ── Events ───────────────────────────────────────────────────
async function loadEvents() {
    try {
        const res = await fetch('/api/parent/events');
        const data = await res.json();
        const logEl = document.getElementById('activity-log');
        const countEl = document.getElementById('event-count');

        if (!data.events || data.events.length === 0) {
            logEl.innerHTML = '<div class="empty-state"><p>No activity yet.</p></div>';
            countEl.textContent = '0 events';
            return;
        }

        countEl.textContent = `${data.events.length} event${data.events.length !== 1 ? 's' : ''}`;
        logEl.innerHTML = data.events.map(evt => {
            const isRevoke = evt.type === 'revoked';
            return `
                <div class="activity-item">
                    <div class="activity-icon ${isRevoke ? 'revoked' : 'registered'}">
                        ${isRevoke ? '✗' : '✓'}
                    </div>
                    <div class="activity-content">
                        <div class="activity-title">${evt.child_name || 'Identity'} — ${isRevoke ? 'Revoked' : 'Registered'}</div>
                        <div class="activity-desc">Contract index: ${evt.contract_index} · Block #${evt.block_number}</div>
                    </div>
                </div>
            `;
        }).join('');

    } catch (err) {
        console.error('Failed to load events:', err);
    }
}

// ── Revocation ───────────────────────────────────────────────
function openRevokeModal(childID, childName) {
    revokeChildID = childID;
    document.getElementById('revoke-child-name').textContent = childName;
    document.getElementById('revoke-password').value = '';
    document.getElementById('revoke-status').style.display = 'none';
    document.getElementById('btn-confirm-revoke').disabled = false;
    document.getElementById('revoke-modal').style.display = 'flex';
}

function closeRevokeModal() {
    document.getElementById('revoke-modal').style.display = 'none';
    revokeChildID = null;
}

async function confirmRevoke() {
    const password = document.getElementById('revoke-password').value;
    const statusEl = document.getElementById('revoke-status');
    const btn = document.getElementById('btn-confirm-revoke');

    if (!password) {
        statusEl.textContent = 'Please enter your password to confirm.';
        statusEl.className = 'revoke-status error';
        statusEl.style.display = 'block';
        return;
    }

    btn.disabled = true;
    btn.textContent = 'Revoking...';
    statusEl.textContent = 'Sending revocation to blockchain...';
    statusEl.className = 'revoke-status pending';
    statusEl.style.display = 'block';

    try {
        const res = await fetch('/api/parent/child/revoke', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ child_id: revokeChildID, password })
        });

        const data = await res.json();
        if (!res.ok) {
            statusEl.textContent = data.error || 'Revocation failed.';
            statusEl.className = 'revoke-status error';
            btn.disabled = false;
            btn.textContent = 'Revoke Identity';
            return;
        }

        statusEl.textContent = 'Identity revoked permanently.';
        statusEl.className = 'revoke-status success';
        btn.textContent = 'Done';

        setTimeout(() => {
            closeRevokeModal();
            refreshDashboard();
        }, 1500);

    } catch (err) {
        statusEl.textContent = 'Network error.';
        statusEl.className = 'revoke-status error';
        btn.disabled = false;
        btn.textContent = 'Revoke Identity';
    }
}

// ── Logout ───────────────────────────────────────────────────
async function logout() {
    await fetch('/api/parent/logout', { method: 'POST' });
    location.reload();
}

// ── Close modal on escape / overlay click ────────────────────
document.addEventListener('keydown', e => {
    if (e.key === 'Escape') closeRevokeModal();
});
document.getElementById('revoke-modal')?.addEventListener('click', e => {
    if (e.target.classList.contains('modal-overlay')) closeRevokeModal();
});

// ── Auto-check session on load ───────────────────────────────
window.addEventListener('load', async () => {
    try {
        const res = await fetch('/api/parent/me');
        if (res.ok) {
            const data = await res.json();
            await onLoginSuccess(data.email || data.phone || 'Parent');
        }
    } catch (e) {
        // Not logged in — show auth screen (already visible by default)
    }
});
```

### 8.4 Create `dashboard/dashboard.css`

Use the same CSS file from the first version of Phase 6 (the styles for summary cards, identity grid, activity log, modal, loading states, responsive breakpoints). Add these additional styles for the new auth and onboarding screens:

```css
/* Add to the beginning of dashboard.css, BEFORE the existing styles */

/* ── Auth Section ──────────────────────────────────────────── */

.auth-section {
    min-height: calc(100vh - 64px - 80px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px 24px;
    background: var(--vq-dark);
}

.auth-card {
    background: var(--vq-dark-mid);
    border: 1px solid rgba(255,255,255,0.08);
    border-radius: var(--vq-radius-lg);
    padding: 40px 36px;
    text-align: center;
    max-width: 440px;
    width: 100%;
    box-shadow: var(--vq-shadow-card);
}

.auth-icon { font-size: 48px; margin-bottom: 16px; }
.auth-card h1 { font-size: 26px; font-weight: 700; color: var(--vq-white); margin-bottom: 8px; }
.auth-card > p { font-size: 14px; color: var(--vq-gray-300); margin-bottom: 24px; }

.auth-tabs, .method-tabs {
    display: flex;
    gap: 0;
    margin-bottom: 20px;
    border: 1px solid rgba(255,255,255,0.1);
    border-radius: var(--vq-radius-full);
    overflow: hidden;
}

.auth-tab, .method-tab {
    flex: 1;
    padding: 10px;
    background: transparent;
    border: none;
    color: var(--vq-gray-400);
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    font-family: var(--vq-font-family);
    transition: background 0.2s, color 0.2s;
}

.auth-tab.active, .method-tab.active {
    background: var(--vq-accent);
    color: var(--vq-dark);
}

.auth-form { text-align: left; }
.auth-form .form-group { margin-bottom: 16px; }
.auth-form label { display: block; font-size: 13px; font-weight: 600; color: var(--vq-gray-300); margin-bottom: 6px; }

.auth-error { color: var(--vq-error); font-size: 13px; margin-top: 16px; }

/* ── Onboarding Section ────────────────────────────────────── */

.onboarding-section {
    min-height: calc(100vh - 64px - 80px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px 24px;
    background: var(--vq-dark);
}

.onboarding-card {
    background: var(--vq-dark-mid);
    border: 1px solid rgba(255,255,255,0.08);
    border-radius: var(--vq-radius-lg);
    padding: 40px 36px;
    text-align: center;
    max-width: 560px;
    width: 100%;
    box-shadow: var(--vq-shadow-card);
}

.onboard-icon { font-size: 52px; margin-bottom: 16px; }
.onboarding-card h1 { font-size: 24px; font-weight: 700; color: var(--vq-white); margin-bottom: 10px; }
.onboarding-card > div > p { font-size: 14px; color: var(--vq-gray-300); margin-bottom: 24px; line-height: 1.5; }

.onboarding-card form { text-align: left; }
.onboarding-card .form-group { margin-bottom: 16px; }
.onboarding-card label { display: block; font-size: 13px; font-weight: 600; color: var(--vq-gray-300); margin-bottom: 6px; }
.onboarding-card select.input { appearance: none; }

/* Verification options */
.verify-options { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 24px; text-align: left; }
.verify-option {
    background: var(--vq-dark-light);
    border: 2px solid rgba(255,255,255,0.08);
    border-radius: var(--vq-radius-md);
    padding: 20px;
    cursor: pointer;
    transition: border-color 0.2s;
    position: relative;
}
.verify-option:hover { border-color: rgba(184,241,42,0.3); }
.verify-option.selected { border-color: var(--vq-accent); }
.verify-option-icon { font-size: 28px; margin-bottom: 10px; }
.verify-option h3 { font-size: 15px; font-weight: 700; color: var(--vq-white); margin-bottom: 6px; }
.verify-option p { font-size: 12px; color: var(--vq-gray-400); line-height: 1.4; }
.verify-badge {
    position: absolute; top: 10px; right: 10px;
    background: rgba(184,241,42,0.15); color: var(--vq-accent);
    font-size: 10px; font-weight: 700; padding: 3px 8px;
    border-radius: var(--vq-radius-full); text-transform: uppercase;
}

.verify-demo-note {
    background: rgba(251,191,36,0.08);
    border: 1px solid rgba(251,191,36,0.2);
    border-radius: var(--vq-radius-sm);
    padding: 14px;
    font-size: 13px;
    color: var(--vq-gray-200);
    margin-bottom: 16px;
    text-align: left;
}

.success-subtext { font-size: 13px !important; color: var(--vq-gray-400) !important; margin-bottom: 24px !important; }

/* ── Add Child Bar ─────────────────────────────────────────── */
.add-child-bar { margin-bottom: 24px; }

/* ── User Info (Nav) ───────────────────────────────────────── */
.user-info { display: flex; align-items: center; gap: 12px; }
.user-email { font-size: 13px; color: var(--vq-gray-300); }

/* ── Pending status badge ──────────────────────────────────── */
.status-badge.pending { background: rgba(251,191,36,0.15); color: var(--vq-warning); }

/* Then include ALL the existing dashboard.css styles from the previous version:
   summary cards, identity grid, activity log, modal, loading states, responsive, etc.
   (copy from the original Phase 6 guide's dashboard.css section) */
```

> **NOTE:** Merge this CSS with the full `dashboard.css` from the original Phase 6 guide (summary cards, identity grid, activity log, modal overlay, loading/empty states, responsive breakpoints). The auth and onboarding styles above go at the TOP of the file; the rest follows below.

---

## STEP 9: Test the Full Parent Flow

**Time: ~20 minutes**

### 9.1 Start all services

**Terminal 1:** `ganache --port 7545`

**Terminal 2:** Deploy contract + start bridge
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle migrate --reset --network development
# Note the contract address

cd ..
go build -o bridge-server ./cmd/bridge/
./bridge-server -contract 0x<ADDR> -client http://127.0.0.1:7545
```

**Terminal 3:** Start Veriqid server
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go run ./cmd/veriqid-server \
  -contract 0x<ADDR> \
  -service "KidsTube" \
  -port 8080 \
  -master-secret "my-hackathon-secret"
```

### 9.2 Test: Parent registration (email)

1. Open `http://localhost:8080/parent`
2. Click **"Create Account"** tab
3. Enter email: `parent@test.com`, password: `testpass123`
4. Click **"Create Account"**

**Expected:** Redirects to onboarding (Add Child screen)

### 9.3 Test: Add child

1. Enter name: `Alex`, select age: `Under 13`
2. Click **"Continue"**

**Expected:** Moves to verification step with two options (Remote Notarization, In-Person)

### 9.4 Test: Simulate verification

1. Click either verification option
2. Click **"Simulate Verification Approval"**

**Expected:**
- Button shows "Verifying..."
- Server terminal shows the on-chain `addID()` transaction
- Success screen appears: "All Set! Alex's identity is verified"

### 9.5 Test: Dashboard

1. Click **"Go to Dashboard"**

**Expected:**
- Summary cards: 1 total, 1 active, 0 revoked
- Child card for "Alex" with status "Active", age "Under 13", truncated MPK
- Activity log shows 1 registration event

### 9.6 Test: Revocation

1. Click **"Revoke Identity"** on Alex's card
2. Modal appears with warning
3. Enter password: `testpass123`
4. Click **"Revoke Identity"**

**Expected:**
- Status shows "Sending revocation to blockchain..."
- Server signs and submits `revokeID()` transaction
- Status shows "Identity revoked permanently."
- Dashboard refreshes: Alex's card shows "Revoked" (red), summary updates

### 9.7 Test: Login after logout

1. Click **"Log out"** in nav
2. Log back in with `parent@test.com` / `testpass123`

**Expected:** Goes directly to dashboard (not onboarding) because Alex already exists

### 9.8 Test: Phone registration

1. Log out, switch to **"Phone"** tab on the **Register** tab
2. Enter phone: `+11234567890`
3. Click **"Send Code"**
4. Check the **server terminal** for `[OTP] Code for +11234567890: XXXXXX`
5. **IMPORTANT:** Make sure the OTP input field is empty (clear any autofill), then type the **exact 6-digit code** from the terminal
6. Click **"Verify Code"**

**Expected:** Logs in and shows onboarding

> **KNOWN ISSUE — Browser Autofill:** Browsers may cache old OTP values in the input field
> and re-submit them on subsequent attempts, even after page refresh. If you get "invalid or
> expired code," check what code is actually being sent by opening F12 → Console and looking
> for `[DEBUG] Sending OTP verify — phone: ... code: ...`. If the code shown doesn't match
> the terminal, clear the input or use an **incognito window** (`Ctrl+Shift+N`).

### 9.9 Test: Phone login (returning user)

1. Log out, switch to the **Login** tab → **Phone** method
2. Enter the same phone: `+11234567890`
3. Click **"Send Code"** — this calls `/api/parent/send-otp` to generate a new OTP
4. Copy the new code from the server terminal
5. Enter the code and click **"Verify Code"**

**Expected:** Logs in and shows dashboard (since child already added)

---

## STEP 10: Troubleshooting

### 10.1 Common Errors

| Error | Cause | Fix |
|-------|-------|-----|
| "email already registered" | Duplicate registration | Log in instead, or clear SQLite: `rm veriqid.db` |
| "phone already registered" | Duplicate phone registration | Use the **Login** tab instead, or `rm veriqid.db` |
| "invalid email or password" | Wrong credentials | Double-check email/password |
| "invalid or expired code" | OTP expired (5-min limit) **OR** browser autofill sending old code | Request new code; check F12 Console for `[DEBUG] Sending OTP verify` to see what code is actually sent; use incognito window |
| `undefined: u2sso.DeriveMPK` | Build error — `DeriveMPK` doesn't exist | Use `u2sso.CreateID(msk)` instead (returns `[]byte` MPK) |
| "on-chain registration failed" | Contract not deployed or wrong address | Redeploy contract, restart server with correct address |
| "not logged in" | Session expired or cookie missing | Log in again |
| "incorrect password" (during revoke) | Re-auth password wrong | Enter the account password, not a child's info |
| "child is not in a revocable state" | Child is pending (not verified) or already revoked | Complete verification first, or check status |
| Dashboard shows stale data | Contract redeployed but DB not cleared | Delete `veriqid.db` and restart server |
| CSS not loading | Incorrect file paths | Verify `dashboard/` directory and `/static/base.css` both exist |
| OTP input shows old code after restart | Browser autofill caching | Hard refresh (`Ctrl+Shift+R`), clear autofill, or use incognito window |

### 10.2 Debugging

**Server logs:** The Go server prints all errors to stderr. Watch the terminal. Key log lines:
- `[OTP] Code for +1...: XXXXXX` — the generated OTP (copy this to enter in the browser)
- `[DEBUG OTP] Login attempt: phone=... code=...` — what the server received from the browser
- `[DEBUG OTP DB] id=... code=... expires=... used=... now_utc=...` — the actual OTP row in the database
- `SUCCESS: Parent registered (ID=...)` — successful registration
- `WARNING: CreateID returned empty mpk` — CGO not available, using placeholder

**Browser console:** F12 → Console shows:
- `[DEBUG] Sending OTP verify — phone: ... code: ...` — confirms what the JS is sending
- `fetch()` errors and JavaScript issues

**SQLite inspection:**
```bash
sqlite3 veriqid.db
.tables
SELECT * FROM parents;
SELECT * FROM children;
SELECT * FROM otp_codes;   -- Check OTP rows: code, expires_at, used
.quit
```

### 10.3 Phone OTP Flow — How It Works

**Registration flow:**
1. Browser sends `POST /api/parent/register` with `{phone}` → server creates account + stores OTP + logs it to terminal
2. Server does **NOT** auto-login for phone (unlike email) — the parent must verify OTP first
3. Browser shows OTP input field; parent copies code from server terminal
4. Browser sends `POST /api/parent/login` with `{phone, code}` → server verifies OTP → creates session

**Login flow (returning user):**
1. Browser sends `POST /api/parent/send-otp` with `{phone}` → server generates new OTP + logs it
2. Browser shows OTP input; parent copies code from terminal
3. Browser sends `POST /api/parent/login` with `{phone, code}` → server verifies OTP → creates session

**Common pitfall:** Browser autofill can inject a stale OTP value into the input field. The `autocomplete="off"` attribute is set on the OTP input, but some browsers ignore it. Always verify what code is being sent by checking the browser console (`[DEBUG] Sending OTP verify`) or server log (`[DEBUG OTP] Login attempt`).

---

## STEP 11: Phase 6 Completion Checklist

```
[x] internal/parent/auth.go — email validation, phone validation, bcrypt hashing, OTP generation
[x] internal/parent/wallet.go — custodial Ethereum key pair generation, AES-256-GCM encryption
[x] internal/parent/verification.go — verifier approval handler (simulated for hackathon)
[x] internal/store/sqlite.go extended — parents, children, otp_codes tables in NewStore()
[x] internal/store/parent_store.go — Parent/Child/OTP structs + all CRUD methods
[x] internal/session/parent_session.go — ParentManager (separate from child SessionManager)
[x] POST /api/parent/register — email+password or phone registration (phone does NOT auto-login)
[x] POST /api/parent/login — email+password or phone+OTP login
[x] POST /api/parent/logout — session destruction
[x] POST /api/parent/send-otp — OTP generation + console logging
[x] POST /api/parent/child/add — add child with name + age bracket + msk via u2sso.CreateID()
[x] POST /api/parent/verify/approve — hackathon simulation of verifier approval
[x] POST /api/parent/child/revoke — one-click revocation with password re-auth
[x] GET /api/parent/children — list children with status
[x] GET /api/parent/events — activity events for parent's children
[x] GET /api/parent/me — current session info
[x] dashboard/index.html — auth screen, onboarding screen, dashboard screen, revoke modal
[x] dashboard/dashboard.js — all client-side logic (no MetaMask, no ethers.js)
[x] dashboard/dashboard.css — auth, onboarding, dashboard styles extending base.css
[x] Auth flow: email+password login/register works end-to-end
[x] Auth flow: phone+OTP login/register works (code printed to server terminal)
[x] Onboarding: add child → verification → success → dashboard
[x] Dashboard: summary cards, child identity cards, activity log
[x] Revocation: modal → password confirmation → status update
[x] Session persistence: refresh page stays logged in, logout works
[x] No MetaMask, no ethers.js, no blockchain terminology visible to parent
[x] Veriqid design: dark teal (#0A1F2F), lime accent (#B8F12A), pill buttons
[x] Responsive layout on smaller screens
```

---

## What Each New File Does (Reference)

### internal/parent/auth.go (~80 lines)
Parent authentication utilities. Email validation (RFC 5322), phone validation (10-15 digits), bcrypt password hashing (cost 12), password strength validation, OTP generation (crypto/rand, 6 digits), email/phone normalization.

### internal/parent/wallet.go (~100 lines)
Custodial Ethereum wallet management. Generates secp256k1 key pairs via `go-ethereum/crypto`, encrypts private keys with AES-256-GCM, decrypts for signing transactions. Encryption key derived from server master secret (hackathon) or parent's password (production). **Note:** `PrivKeyHexToECDSA` returns `*ecdsa.PrivateKey` (requires `"crypto/ecdsa"` import), uses `crypto.ToECDSA()` not `crypto.HexToECDSA()`.

### internal/parent/verification.go (~50 lines)
Verifier integration. For hackathon: a simulated approval endpoint that calls `addID()` on the contract using the admin key. For production: would accept signed attestations from remote notaries or in-person verifiers.

### internal/store/sqlite.go additions
Schema extension: three new `CREATE TABLE` statements (parents, children, otp_codes) added to `NewStore()` after the existing Phase 4 tables.

### internal/store/parent_store.go (~210 lines)
`Parent` and `Child` structs + all CRUD methods for parent accounts, children, and OTP codes. **Note:** Uses `UpdateParentLastLogin` (not `UpdateLastLogin`) to avoid collision with the existing Phase 4 method that takes `spkHex`.

### internal/session/parent_session.go (~120 lines)
`ParentManager` — separate session manager for parent sessions (cookie: `veriqid_parent_session`). Runs alongside the existing child `SessionManager` (cookie: `veriqid_session`) without interference. HMAC-based session tokens, 24-hour expiry, 5-minute cleanup ticker.

### dashboard/index.html (~260 lines)
Single-page parent portal with three screens: Auth (login/register with email/phone tabs), Onboarding (add child → verification → success), Dashboard (summary stats, child cards, activity log). Includes revoke confirmation modal with password re-authentication. OTP input has `autocomplete="off"` to prevent browser autofill issues.

### dashboard/dashboard.js (~340 lines)
Client-side logic for the parent portal. All communication via `fetch()` to `/api/parent/*` endpoints. No blockchain libraries, no MetaMask — all chain interaction is server-side. Handles auth tab switching, OTP flow (with field clearing between attempts), child creation, verification simulation, dashboard rendering, revocation with password confirmation, auto-session-check on page load. Includes debug logging (`console.log`) for OTP verification troubleshooting.

### dashboard/dashboard.css (~480 lines)
Styles for auth card, onboarding flow, verification option cards, and all dashboard components (summary grid, identity cards, activity log, modal). Extends `base.css` with dashboard-specific layouts. Responsive at 768px breakpoint.

### cmd/veriqid-server/main.go modifications
Added `-master-secret` flag, `ParentSessions *session.ParentManager`, 10 parent API handlers, dashboard file server, and `/parent` route. Key implementation details:
- `handleAddChild` uses `u2sso.CreateID(msk)` (not `DeriveMPK`) with fallback to placeholder if CGO fails
- `handleParentRegister` does NOT auto-login for phone registration (OTP verification required first)
- `handleVerifyApprove` uses child.ID as placeholder contract index (hackathon simulation)
- `handleRevokeChild` requires password re-authentication

---

## How the Dashboard Connects to Other Phases

### Phase 3 (Enhanced Smart Contract)
The server calls `addID()` (registration), `revokeID()` (revocation), `getState()` (status checks), and queries `IDRegistered`/`IDRevoked` events — all through the Go contract bindings from Phase 3. The parent never interacts with the contract directly.

### Phase 4 (Veriqid Server)
Phase 6 extends the Phase 4 server with new routes and database tables. The SQLite store, session system, and server architecture from Phase 4 are reused directly.

### Phase 5 (Browser Extension + Design System)
The dashboard reuses `base.css` for consistent Veriqid branding. When a parent completes onboarding, the next step is installing the browser extension on the child's device — the extension receives the child's msk from the server (transferred securely during setup).

### Phase 7 (Demo Platform)
The hackathon demo script becomes: parent creates account → verifies child → child signs up on KidsTube (via extension) → parent sees registration in dashboard → parent revokes → child loses access. The parent dashboard is the "control center" that makes the demo story compelling.

---

## Next Steps: Phase 7 — Demo Platform

With Phase 6 complete, both sides of the Veriqid story are functional: children sign up anonymously on platforms (extension), and parents monitor and control those identities (dashboard). Phase 7 creates the polished "KidsTube" demo platform — a mock video platform that integrates Veriqid, showing judges the complete end-to-end experience.
