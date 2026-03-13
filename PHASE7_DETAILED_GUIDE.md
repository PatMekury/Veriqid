# Veriqid Phase 7: Demo Platform ("KidsTube") — Complete Detailed Guide

**Time estimate: ~4 hours**
**Goal: Build a mock children's video platform ("KidsTube") that integrates Veriqid end-to-end — showing a "Sign up with Veriqid" button, proof verification in <2 seconds, pseudonymous accounts, age-gated content, and unlinkability across platforms. This is the showcase piece for the hackathon demo: parent verifies child → child signs up on KidsTube via extension → parent sees registration in dashboard → parent revokes → child loses access.**

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

### The Missing Piece

In Phases 1–6, you built all the infrastructure:

1. **Phase 1:** Crypto library + proof-of-concept (CLI creates identities, server verifies proofs)
2. **Phase 2:** Bridge API (HTTP wrapper around CGO so the browser can call crypto functions)
3. **Phase 3:** Enhanced smart contract (Veriqid.sol — verifier registry, events, age brackets, owner fix)
4. **Phase 4:** Veriqid server (SQLite persistence, sessions, configurable service name, age-verification endpoints)
5. **Phase 5:** Browser extension (auto-detects Veriqid forms, calls bridge, auto-fills proofs)
6. **Phase 6:** Parent dashboard (parent creates account, verifies child, monitors activity, revokes identity)

But there's no **consumer-facing platform** to demo. The Phase 4 server shows signup/login forms, but they look like developer tools — raw field names like "SPK," "Membership Proof," "Decoy Size." For the hackathon demo, you need a **real-looking platform** that a judge can immediately understand: "This is a children's video site. A kid is signing up. The parent can see it. The parent can revoke it."

### ASC Formal Model Alignment

Veriqid implements the **Anonymous Self-Credentials (ASC)** protocol. Here's how KidsTube maps to the formal model:

| ASC Formal Term | Veriqid Implementation | Role in KidsTube |
|---|---|---|
| **Setup()** | Veriqid.sol deployment + secp256k1/Boquila curve params | Contract + crypto lib initialized at server start |
| **Gen(sk) → ID** | `MSK` (32-byte secret) → `MPK` (compressed public key via `DerivePublicKey`) | Parent creates child identity; MPK stored on-chain |
| **sk** (secret key) | `MSK` — 32 bytes, derived from 12-word mnemonic via SHA-256 | Stored in browser extension (Phase 5 or portable extension) |
| **ID** (identity, anonymity set member) | `MPK` — 33-byte compressed secp256k1 point, on-chain | Registered via `registerID()` in Veriqid.sol |
| **Pseudonym (a)** | `SPK` — deterministic per (MSK, service_name_hash) | User's KidsTube identity; stored in `users.spk_hex` |
| **Nullifier (nul)** | `SPK` (serves dual role — see note below) | Enforced via `UNIQUE` constraint on `spk_hex` |
| **Prove(sk, S, m) → (a, nul, π)** | Bridge `/api/identity/register` → `RegistrationProof()` | Extension calls bridge; KidsTube receives proof in form |
| **Verify(S, m, a, nul, π) → bool** | `u2sso.RegistrationVerify()` in KidsTube server | Server verifies ring membership proof at signup |
| **Anonymity Set (S)** | All active MPKs on-chain (`GetallActiveIDfromContract()`) | Fetched by KidsTube server at proof verification time |

> **Note — SPK Serves Dual Role (Pseudonym + Nullifier):**
> The formal ASC model outputs Prove() as a triple `(a, nul, π)` where the pseudonym `a` and nullifier `nul` are separate values. In Veriqid, the SPK serves **both** roles simultaneously. It acts as the pseudonym (the child's stable identifier on KidsTube) **and** the nullifier (the `UNIQUE` constraint on `spk_hex` prevents double-registration, providing Sybil resistance). This works because SPK is deterministic per `(MSK, service_name_hash)` — so it naturally deduplicates. This is a valid simplification, not a flaw, and is arguably more elegant for a children's identity system where one-identity-per-service is exactly the desired behavior.

**ASC Security Properties Satisfied by KidsTube:**

| Property | How KidsTube Satisfies It |
|---|---|
| **Correctness** | Honest child with valid MSK + on-chain MPK always generates valid proof; bridge deterministically derives SPK |
| **Robustness** | Contract `registerID()` appends to array — one user can't block another's registration |
| **Unforgeability** | Ring proof relies on discrete-log hardness of secp256k1; no valid proof without valid MSK |
| **Sybil-Resistance** | SPK is deterministic per (MSK, service); `UNIQUE` constraint on `spk_hex` enforces one-account-per-child |
| **Anonymity** | Ring membership proof (`RegistrationProof`) hides which MPK from the anonymity set belongs to prover |
| **Unlinkability** | `SPK = f(MSK, service_name_hash)` is one-way; KidsTube's SPK reveals nothing about SPK on other platforms |

### What Phase 7 Builds

```
┌─────────────────────────────────────────────────────────────────┐
│  KIDSTUBE (Browser — localhost:3000)                             │
│                                                                  │
│  ┌─────────────┐  ┌───────────────────┐  ┌───────────────────┐  │
│  │ Landing     │  │ "Sign up with     │  │ Protected Content │  │
│  │ Page        │→ │  Veriqid" Flow    │→ │ (Video Grid)      │  │
│  │ (hero +     │  │ (extension auto-  │  │ Age-appropriate   │  │
│  │  features)  │  │  fills proof)     │  │ pseudonymous      │  │
│  └─────────────┘  └───────────────────┘  └───────┬───────────┘  │
│                                                   │              │
│  ┌─────────────┐  ┌───────────────────┐          │              │
│  │ Profile     │  │ Login with        │          │              │
│  │ Page        │  │ Veriqid           │◄─────────┘              │
│  │ (pseudonym, │  │ (re-auth via      │                         │
│  │  no PII)    │  │  extension)       │                         │
│  └─────────────┘  └───────────────────┘                         │
│                                                                  │
└──────────────────────────────┬──────────────────────────────────┘
                               │
         User sees a normal    │  Behind the scenes:
         video platform        │  Veriqid crypto
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│  KIDSTUBE SERVER (Go backend — cmd/demo-platform/main.go)       │
│                                                                  │
│  ┌────────────────┐  ┌──────────────────┐  ┌────────────────┐  │
│  │ Veriqid        │  │ Session          │  │ Content        │  │
│  │ Integration    │  │ Management       │  │ Serving        │  │
│  │ (challenge +   │  │ (cookie-based,   │  │ (video grid,   │  │
│  │  verify via    │  │  pseudonymous)   │  │  profile,      │  │
│  │  u2sso pkg)    │  │                  │  │  age-gated)    │  │
│  └───────┬────────┘  └────────┬─────────┘  └───────┬────────┘  │
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

### The Demo Story (What Judges See)

**This is the 60-second demo script for the KidsTube portion:**

1. **Parent creates account** (Phase 6 dashboard at `localhost:8080/parent`) → adds child "Alex"
2. **Parent receives 12-word mnemonic phrase** → gives it to child (or child pastes into portable extension)
3. **Child opens KidsTube** (`localhost:3000`) → sees a colorful video platform landing page
4. **Child clicks "Sign up with Veriqid"** → the browser extension detects the form, auto-fills proof values, submits
5. **KidsTube shows: "Welcome, Alex!"** → child sees age-appropriate video thumbnails, a pseudonymous profile
6. **Parent checks dashboard** → sees "Alex registered on KidsTube" in the activity log
7. **Parent clicks "Revoke"** → identity is revoked on-chain
8. **Child refreshes KidsTube** → "Your Veriqid identity has been revoked. Contact your parent."

The key insight for judges: **KidsTube never learned the child's real name, age, or parent info.** It only knows "this is a verified child with pseudonym `spk_abc123`." Yet the parent has full control.

### Why a Separate Server (Not Just Phase 4)?

Phase 4's server IS a Veriqid-integrated platform, but it was designed as a developer demo — it shows the raw crypto values and has a generic UI. KidsTube is a **consumer demo** — it hides all the crypto behind a polished UI and focuses on the user experience.

More importantly, running KidsTube on a **different port** (`localhost:3000`) from the Veriqid server (`localhost:8080`) proves a critical point: **Veriqid works across independent platforms.** If you had time, you could spin up a second demo platform ("KidsChat" on `localhost:3001`) and show that the same child gets a **different, unlinkable pseudonym** on each platform — directly demonstrating the ASC Unlinkability property.

| | Phase 4 Server (`:8080`) | KidsTube (`:3000`) |
|---|---|---|
| **Purpose** | Veriqid infrastructure demo | Consumer platform demo |
| **Audience** | Developers evaluating Veriqid | End users (children, parents, judges) |
| **Shows** | Raw proof values, crypto details | Polished video platform UI |
| **Service name** | Configurable (`-service` flag) | Always "KidsTube" |
| **UI** | Veriqid branding (teal/lime) | KidsTube branding (bright, kid-friendly) |
| **Runs on** | `localhost:8080` | `localhost:3000` |

### Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| **Port** | `localhost:3000` | Different from Veriqid server (8080) to prove cross-platform capability |
| **Service name** | `"KidsTube"` (SHA-256 hashed) | Deterministic — same child always gets the same SPK (pseudonym/nullifier) on KidsTube |
| **Backend** | Go (reuses `pkg/u2sso`) | Same crypto library, consistent with rest of project |
| **Frontend** | Static HTML/CSS/JS | No build tools needed, works in any browser |
| **Content** | Mock video thumbnails (CSS placeholders) | No real videos — just enough to look like a platform |
| **Sessions** | Cookie-based (same pattern as Phase 4) | Proven pattern, pseudonymous |
| **Database** | SQLite (`kidstube.db`) | Separate from `veriqid.db` — proves platform independence |
| **Design** | Bright, kid-friendly colors | Visually distinct from the Veriqid teal/lime design system |
| **Extension integration** | Hidden `veriqid-*` fields in signup form | Phase 5 content script (or portable extension) auto-detects these field IDs |
| **Contract binding** | `*u2sso.Veriqid` / `u2sso.NewVeriqid()` | Uses the Veriqid.go binding generated from Veriqid.sol (not the legacy U2SSO.go) |
| **Revocation** | Real on-chain check via `contractInst.GetState(index)` | Stores `contract_index` at signup; queries contract on every protected page load |
| **Activity reporting** | POST to Veriqid server `/api/platform/activity` | Parent dashboard shows "Alex registered on KidsTube" in activity log |

---

## PREREQUISITES

Before starting Phase 7, you must have Phases 1–6 fully complete:

```
[x] libsecp256k1 built and installed with --enable-module-ringcip
[x] bridge/bridge.go compiles (go build -o bridge-server ./cmd/bridge/)
[x] Veriqid.sol deployed with events, verifier registry, age brackets (Phase 3)
[x] cmd/veriqid-server running with SQLite, sessions, templates (Phase 4)
[x] Browser extension functional — detects forms, auto-fills proofs (Phase 5)
    OR portable extension (extension-portable) with mnemonic-based key
[x] Parent dashboard functional — create account, verify child, monitor, revoke (Phase 6)
[x] static/base.css contains full Veriqid design system (Phase 5)
[x] Ganache running on port 7545
```

You'll also need:
- The contract address from your latest deployment
- At least 2 registered identities (from Phase 1 or Phase 6 onboarding)
- The bridge running on `localhost:9090`
- The Veriqid server running on `localhost:8080`

> **Extension Options:** KidsTube works with either:
> - **Phase 5 extension** (`extension/`) — uses file-based keypath
> - **Portable extension** (`extension-portable/`) — uses mnemonic-derived MSK via `msk_hex`
>
> Both extensions detect the same hidden `veriqid-*` form fields and fill them the same way. The portable extension additionally fills the `veriqid-contract-index` field, which enables revocation checking.

---

## STEP 1: Create the Demo Platform Directory Structure

**Time: ~10 minutes**

### 1.1 Navigate to the Veriqid root

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"
```

### 1.2 Create the demo-platform directory

```bash
mkdir -p demo-platform/static/css
mkdir -p demo-platform/static/js
mkdir -p demo-platform/static/img
mkdir -p demo-platform/templates
```

### 1.3 Verify the structure

```bash
find demo-platform -type d
```

**Expected output:**

```
demo-platform
demo-platform/static
demo-platform/static/css
demo-platform/static/js
demo-platform/static/img
demo-platform/templates
```

### 1.4 Create the server entry point directory

```bash
mkdir -p cmd/demo-platform
```

Your updated Veriqid folder structure now includes:

```
Veriqid/
├── cmd/
│   ├── client/main.go          (Phase 1)
│   ├── server/main.go          (Phase 1)
│   ├── bridge/main.go          (Phase 2)
│   ├── veriqid-server/main.go  (Phase 4)
│   └── demo-platform/main.go   (Phase 7: NEW)
├── bridge/bridge.go            (Phase 2)
├── pkg/u2sso/
│   ├── u2ssolib.go
│   ├── U2SSO.go                (legacy binding — do NOT use)
│   └── Veriqid.go              (correct binding — use this)
├── contracts/
├── extension/                  (Phase 5)
├── extension-portable/         (Portable key extension)
├── dashboard/                  (Phase 6)
├── internal/mnemonic/          (Mnemonic key generation)
├── demo-platform/              (Phase 7: NEW)
│   ├── static/
│   │   ├── css/kidstube.css
│   │   ├── js/kidstube.js
│   │   └── img/
│   └── templates/
│       ├── landing.html
│       ├── signup.html
│       ├── home.html
│       ├── profile.html
│       └── revoked.html
├── static/                     (Phase 1/5 web UI)
└── go.mod
```

---

## STEP 2: Build the KidsTube Go Server

**Time: ~45 minutes**

### 2.1 Understand what the server does

The KidsTube server is a **standalone Go application** that:

1. Serves a landing page with a "Sign up with Veriqid" button
2. Generates challenges for the signup/login forms (using `u2sso.CreateChallenge()`)
3. Injects the challenge and service name into hidden form fields (detected by the extension)
4. Receives proof submissions from the form (auto-filled by the extension)
5. Verifies proofs using `u2sso.RegistrationVerify()` and `u2sso.AuthVerify()`
6. Creates pseudonymous sessions (cookie-based, stores only the SPK — no real name, no PII)
7. Serves age-appropriate mock content behind the auth wall
8. Checks identity status on-chain (revocation detection via `contractInst.GetState()`)
9. Reports platform activity to the Veriqid server (parent dashboard visibility)

**ASC mapping:** Steps 2–4 implement the challenge generation and message preparation for `Prove()`. Step 5 implements `Verify()`. Step 8 enforces post-revocation denial (the parent calls `revokeID()` which sets `active=false` on-chain).

### 2.2 Create cmd/demo-platform/main.go

```bash
cat > cmd/demo-platform/main.go << 'GOEOF'
package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/mattn/go-sqlite3"

	u2sso "github.com/patmekury/veriqid/pkg/u2sso"
)

// ─── Configuration ─────────────────────────────────────────────

var (
	contractAddr  string
	ethClientURL  string
	port          string
	serviceName   string
	serviceHash   string
	baseDir       string
	veriqidServer string // URL of the Veriqid server (Phase 6) for activity reporting
)

// ─── Session Store (in-memory for hackathon) ───────────────────
// Sessions are pseudonymous: store only the SPK (ASC pseudonym/nullifier)
// and a display name. No PII ever touches this store.

type Session struct {
	SPKHex    string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session // token → session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

func (s *SessionStore) Create(spkHex, username string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate session token from SPK + timestamp
	tokenSrc := fmt.Sprintf("%s:%d", spkHex, time.Now().UnixNano())
	h := sha256.Sum256([]byte(tokenSrc))
	token := hex.EncodeToString(h[:16])

	s.sessions[token] = &Session{
		SPKHex:    spkHex,
		Username:  username,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}
	return token
}

func (s *SessionStore) Get(token string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[token]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return nil
	}
	return sess
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// ─── Database (SQLite — separate from veriqid.db) ─────────────

var db *sql.DB

func initDB() {
	var err error
	dbPath := filepath.Join(baseDir, "kidstube.db")
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Create tables
	// spk_hex is UNIQUE — this enforces ASC Sybil-Resistance:
	// one SPK (pseudonym/nullifier) per identity per platform.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			spk_hex         TEXT NOT NULL UNIQUE,
			username        TEXT NOT NULL,
			age_bracket     INTEGER DEFAULT 0,
			contract_index  INTEGER,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login      DATETIME
		);

		CREATE TABLE IF NOT EXISTS challenges (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			challenge    TEXT NOT NULL UNIQUE,
			created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			used         BOOLEAN DEFAULT FALSE
		);
	`)
	// Migration: add contract_index column if upgrading from old schema
	db.Exec("ALTER TABLE users ADD COLUMN contract_index INTEGER")
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
}

// ─── Contract Interface ────────────────────────────────────────
// Uses the Veriqid.go binding (generated from Veriqid.sol).
// Do NOT use the legacy U2SSO.go binding.

var (
	ethClient    *ethclient.Client
	contractInst *u2sso.Veriqid
)

func initContract() {
	var err error
	ethClient, err = ethclient.Dial(ethClientURL)
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	addr := common.HexToAddress(contractAddr)
	contractInst, err = u2sso.NewVeriqid(addr, ethClient)
	if err != nil {
		log.Fatalf("Failed to load contract: %v", err)
	}

	// Verify connection — query the anonymity set size (number of registered IDs)
	size, err := u2sso.GetIDfromContract(contractInst)
	if err != nil {
		log.Fatalf("Failed to query contract: %v", err)
	}
	log.Printf("Connected to contract at %s — %d identities registered", contractAddr, size)
}

// ─── Template Helpers ──────────────────────────────────────────

var templates *template.Template

func loadTemplates() {
	tmplDir := filepath.Join(baseDir, "demo-platform", "templates")
	var err error
	templates, err = template.ParseGlob(filepath.Join(tmplDir, "*.html"))
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
}

// ─── HTTP Handlers ─────────────────────────────────────────────

var sessionStore = NewSessionStore()

// Landing page — public
func handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Check if user is already logged in
	cookie, err := r.Cookie("kidstube_session")
	if err == nil {
		sess := sessionStore.Get(cookie.Value)
		if sess != nil {
			http.Redirect(w, r, "/home", http.StatusSeeOther)
			return
		}
	}

	templates.ExecuteTemplate(w, "landing.html", map[string]interface{}{
		"ServiceName": serviceName,
	})
}

// Signup page — generates challenge (ASC message m), serves form with hidden Veriqid fields.
// The extension detects these fields and calls Prove(sk, S, m) via the bridge.
func handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleSignupSubmit(w, r)
		return
	}

	challenge := u2sso.CreateChallenge()
	challengeHex := hex.EncodeToString(challenge)

	// Store challenge for replay protection
	db.Exec("INSERT INTO challenges (challenge) VALUES (?)", challengeHex)

	templates.ExecuteTemplate(w, "signup.html", map[string]interface{}{
		"Challenge":    challengeHex,
		"ServiceName":  serviceHash,
		"ServiceLabel": serviceName,
		"IsLogin":      false,
	})
}

// Signup form submission — implements ASC Verify(S, m, a, nul, π).
// Receives the proof (π), SPK (a/nul), challenge (m), and verifies
// against the on-chain anonymity set (S).
func handleSignupSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	proofHex := r.FormValue("veriqid-proof")
	spkHex := r.FormValue("veriqid-spk")               // ASC: pseudonym (a) AND nullifier (nul)
	challengeHex := r.FormValue("veriqid-challenge")     // ASC: message (m)
	snameHex := r.FormValue("veriqid-service-name")
	nStr := r.FormValue("veriqid-decoy-size")
	contractIndexStr := r.FormValue("veriqid-contract-index")
	username := r.FormValue("username")

	// Validate required fields
	if proofHex == "" || spkHex == "" || challengeHex == "" {
		http.Error(w, "Missing proof data. Is the Veriqid extension installed?", http.StatusBadRequest)
		return
	}

	if username == "" {
		username = "KidsTuber_" + spkHex[:8] // Default pseudonym from SPK prefix
	}

	// Check challenge was issued by us and hasn't been used
	var used bool
	err := db.QueryRow("SELECT used FROM challenges WHERE challenge = ?", challengeHex).Scan(&used)
	if err != nil {
		log.Printf("Challenge not found: %s", challengeHex)
		http.Error(w, "Invalid or expired challenge. Please try again.", http.StatusBadRequest)
		return
	}
	if used {
		http.Error(w, "Challenge already used (replay attack prevented). Please try again.", http.StatusBadRequest)
		return
	}

	// Mark challenge as used
	db.Exec("UPDATE challenges SET used = TRUE WHERE challenge = ?", challengeHex)

	// ASC Sybil-Resistance check: SPK (pseudonym/nullifier) must be unique per platform.
	// Since SPK = f(MSK, service_name_hash) is deterministic, the same child always
	// produces the same SPK on KidsTube — UNIQUE constraint prevents double-registration.
	var existingID int
	err = db.QueryRow("SELECT id FROM users WHERE spk_hex = ?", spkHex).Scan(&existingID)
	if err == nil {
		http.Error(w, "This identity is already registered on KidsTube. One account per child!", http.StatusConflict)
		return
	}

	// Decode challenge and service name from hex
	challengeBytes, err := hex.DecodeString(challengeHex)
	if err != nil {
		http.Error(w, "Invalid challenge format", http.StatusBadRequest)
		return
	}

	snameBytes, err := hex.DecodeString(snameHex)
	if err != nil {
		http.Error(w, "Invalid service name format", http.StatusBadRequest)
		return
	}

	spkBytes, err := hex.DecodeString(spkHex)
	if err != nil {
		http.Error(w, "Invalid SPK format", http.StatusBadRequest)
		return
	}

	// Parse ring size
	var currentN int
	fmt.Sscanf(nStr, "%d", &currentN)
	if currentN < 2 {
		currentN = 2 // Minimum ring size
	}

	// Calculate m from N (ring size = 2^m)
	currentM := 0
	temp := currentN
	for temp > 1 {
		temp >>= 1
		currentM++
	}

	// Fetch all active IDs from the contract — this is the ASC anonymity set S.
	// The ring membership proof proves the prover's ID is in S without revealing which one.
	idList, err := u2sso.GetallActiveIDfromContract(contractInst)
	if err != nil {
		log.Printf("Failed to fetch IDs from contract: %v", err)
		http.Error(w, "Failed to verify identity — contract unavailable", http.StatusInternalServerError)
		return
	}

	// ─── ASC Verify(S, m, a, nul, π) ───
	// Verifies the ring membership proof against the anonymity set.
	// This confirms: (1) the prover knows a secret key sk such that Gen(sk) is in S,
	// (2) the pseudonym/nullifier a was correctly derived for this service.
	valid := u2sso.RegistrationVerify(proofHex, currentM, 2, snameBytes, challengeBytes, idList, spkBytes)

	if !valid {
		log.Printf("Registration proof INVALID for SPK: %s", spkHex[:16])
		http.Error(w, "Identity verification failed. The proof is invalid.", http.StatusForbidden)
		return
	}

	log.Printf("Registration proof VALID for SPK: %s", spkHex[:16])

	// Age bracket defaults to 0 (unknown) — in production, query from contract
	ageBracket := 0

	// Parse contract_index from the extension (the extension knows which on-chain identity it used)
	var contractIndex *int64
	if contractIndexStr != "" {
		if idx, err := strconv.ParseInt(contractIndexStr, 10, 64); err == nil {
			contractIndex = &idx
			log.Printf("Contract index provided by extension: %d", idx)
		}
	}

	// If the extension didn't provide contract_index, try to infer it
	// by checking all on-chain IDs. This is a fallback — normally the extension provides it.
	if contractIndex == nil {
		log.Printf("WARNING: No contract_index provided by extension. Attempting contract lookup...")
		idxFromContract := findContractIndexBySPK(spkHex, snameBytes, challengeBytes)
		if idxFromContract >= 0 {
			contractIndex = &idxFromContract
			log.Printf("Found contract index via lookup: %d", idxFromContract)
		} else {
			log.Printf("WARNING: Could not determine contract index — revocation checks will be unavailable for this user")
		}
	}

	// Store the user with contract_index for revocation checking
	_, err = db.Exec(
		"INSERT INTO users (spk_hex, username, age_bracket, contract_index) VALUES (?, ?, ?, ?)",
		spkHex, username, ageBracket, contractIndex,
	)
	if err != nil {
		log.Printf("Failed to store user: %v", err)
		http.Error(w, "Account creation failed", http.StatusInternalServerError)
		return
	}

	// Create session
	token := sessionStore.Create(spkHex, username)
	http.SetCookie(w, &http.Cookie{
		Name:     "kidstube_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   7200, // 2 hours
		SameSite: http.SameSiteLaxMode,
	})

	log.Printf("User '%s' registered on KidsTube (SPK: %s..., contractIndex: %v)", username, spkHex[:16], contractIndex)

	// Report platform registration to Veriqid server (Parent Portal visibility)
	go reportPlatformActivity(spkHex, contractIndex, "registered")

	// Redirect to home
	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

// Login page — generates challenge, serves login form
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleLoginSubmit(w, r)
		return
	}

	challenge := u2sso.CreateChallenge()
	challengeHex := hex.EncodeToString(challenge)

	db.Exec("INSERT INTO challenges (challenge) VALUES (?)", challengeHex)

	templates.ExecuteTemplate(w, "signup.html", map[string]interface{}{
		"Challenge":    challengeHex,
		"ServiceName":  serviceHash,
		"ServiceLabel": serviceName,
		"IsLogin":      true,
	})
}

// Login form submission — verify Boquila auth proof (ASC re-authentication).
// Unlike registration, login uses a lightweight signature proof (AuthVerify)
// instead of a full ring membership proof. This proves the user controls the
// same MSK (sk) that produced the stored SPK — without re-proving ring membership.
func handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	signatureHex := r.FormValue("veriqid-signature")
	spkHex := r.FormValue("veriqid-spk")
	challengeHex := r.FormValue("veriqid-challenge")
	_ = r.FormValue("veriqid-service-name") // read but not needed for auth verify directly

	if signatureHex == "" || spkHex == "" || challengeHex == "" {
		http.Error(w, "Missing auth data. Is the Veriqid extension installed?", http.StatusBadRequest)
		return
	}

	// Validate challenge
	var used bool
	err := db.QueryRow("SELECT used FROM challenges WHERE challenge = ?", challengeHex).Scan(&used)
	if err != nil || used {
		http.Error(w, "Invalid or expired challenge", http.StatusBadRequest)
		return
	}
	db.Exec("UPDATE challenges SET used = TRUE WHERE challenge = ?", challengeHex)

	// Check that this SPK (pseudonym) is registered on KidsTube
	var username string
	err = db.QueryRow("SELECT username FROM users WHERE spk_hex = ?", spkHex).Scan(&username)
	if err != nil {
		http.Error(w, "No account found for this identity on KidsTube", http.StatusNotFound)
		return
	}

	// Decode values
	challengeBytes, _ := hex.DecodeString(challengeHex)
	snameBytes, _ := hex.DecodeString(serviceHash)
	spkBytes, _ := hex.DecodeString(spkHex)

	// ─── VERIFY THE AUTH PROOF ───
	valid := u2sso.AuthVerify(signatureHex, snameBytes, challengeBytes, spkBytes)

	if !valid {
		log.Printf("Auth proof INVALID for SPK: %s", spkHex[:16])
		http.Error(w, "Authentication failed", http.StatusForbidden)
		return
	}

	log.Printf("Auth proof VALID for user '%s'", username)

	// Update last login
	db.Exec("UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE spk_hex = ?", spkHex)

	// Create session
	token := sessionStore.Create(spkHex, username)
	http.SetCookie(w, &http.Cookie{
		Name:     "kidstube_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   7200,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

// Home page — protected, shows mock video content
func handleHome(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if sess == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Check if identity is still active on-chain (revocation detection)
	revoked := checkRevocation(sess.SPKHex)

	if revoked {
		// Clear session
		cookie, _ := r.Cookie("kidstube_session")
		if cookie != nil {
			sessionStore.Delete(cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:   "kidstube_session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		templates.ExecuteTemplate(w, "revoked.html", nil)
		return
	}

	templates.ExecuteTemplate(w, "home.html", map[string]interface{}{
		"Username":    sess.Username,
		"SPKShort":    sess.SPKHex[:16] + "...",
		"ServiceName": serviceName,
	})
}

// Profile page — shows pseudonymous account info
func handleProfile(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if sess == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	templates.ExecuteTemplate(w, "profile.html", map[string]interface{}{
		"Username":    sess.Username,
		"SPKHex":      sess.SPKHex,
		"SPKShort":    sess.SPKHex[:16] + "...",
		"CreatedAt":   sess.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
		"ServiceName": serviceName,
	})
}

// Logout
func handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("kidstube_session")
	if err == nil {
		sessionStore.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "kidstube_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// API: Check session status (for extension communication)
func handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sess := getSession(r)
	if sess == nil {
		fmt.Fprintf(w, `{"authenticated": false}`)
		return
	}

	fmt.Fprintf(w, `{"authenticated": true, "username": "%s", "spk_short": "%s"}`,
		sess.Username, sess.SPKHex[:16])
}

// ─── Helpers ───────────────────────────────────────────────────

func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("kidstube_session")
	if err != nil {
		return nil
	}
	return sessionStore.Get(cookie.Value)
}

// checkRevocation queries the smart contract to see if the identity's MPK
// has been revoked. Uses the contract_index stored at signup time.
// When a parent calls revokeID(index) in the dashboard, this detects it.
func checkRevocation(spkHex string) bool {
	// Look up the contract_index for this user
	var contractIndex sql.NullInt64
	err := db.QueryRow("SELECT contract_index FROM users WHERE spk_hex = ?", spkHex).Scan(&contractIndex)
	if err != nil {
		log.Printf("checkRevocation: user not found for SPK %s...: %v", spkHex[:16], err)
		return false // Can't check → assume not revoked
	}

	if !contractIndex.Valid {
		// No contract_index stored — extension didn't provide it at signup
		log.Printf("checkRevocation: no contract_index for SPK %s... — cannot check on-chain state", spkHex[:16])
		return false
	}

	// Query the smart contract: getState(index) returns true if ACTIVE
	index := big.NewInt(contractIndex.Int64)
	active, err := contractInst.GetState(nil, index)
	if err != nil {
		log.Printf("checkRevocation: contract query failed for index %d: %v", contractIndex.Int64, err)
		return false // Contract error → assume not revoked (fail open for demo)
	}

	if !active {
		log.Printf("REVOCATION DETECTED: SPK %s... (contractIndex=%d) is no longer active on-chain",
			spkHex[:16], contractIndex.Int64)

		// Report revocation event to Veriqid server (so Parent Portal can see it)
		go reportPlatformActivity(spkHex, func() *int64 { v := contractIndex.Int64; return &v }(), "revoked_detected")

		return true // Identity has been revoked by the parent
	}

	return false
}

// findContractIndexBySPK is a fallback that tries to determine the contract index
// when the extension didn't provide it. This iterates all on-chain IDs, which is
// only feasible for small sets (hackathon demo). Returns -1 if not found.
func findContractIndexBySPK(spkHex string, snameBytes, challengeBytes []byte) int64 {
	// For the demo, we can't directly map SPK → contract index without knowing the MPK.
	// The privacy architecture intentionally prevents this linkage (ASC Anonymity property).
	// This function exists as a placeholder — the real solution is to have the extension
	// provide the contract_index at registration time.
	return -1
}

// reportPlatformActivity sends a notification to the Veriqid server so the
// parent dashboard can show platform-specific events ("Alex registered on KidsTube").
func reportPlatformActivity(spkHex string, contractIndex *int64, eventType string) {
	if veriqidServer == "" {
		log.Printf("Platform activity reporting skipped — no Veriqid server URL configured")
		return
	}

	payload := map[string]interface{}{
		"service_name": serviceName,
		"spk_hex":      spkHex,
		"event_type":   eventType,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	if contractIndex != nil {
		payload["contract_index"] = *contractIndex
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("reportPlatformActivity: marshal error: %v", err)
		return
	}

	url := veriqidServer + "/api/platform/activity"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("reportPlatformActivity: POST to %s failed: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Platform activity reported: %s (SPK: %s..., service: %s)", eventType, spkHex[:16], serviceName)
	} else {
		log.Printf("reportPlatformActivity: server returned %d", resp.StatusCode)
	}
}

// ─── Main ──────────────────────────────────────────────────────

func main() {
	flag.StringVar(&contractAddr, "contract", "", "Veriqid contract address (0x...)")
	flag.StringVar(&ethClientURL, "client", "http://127.0.0.1:7545", "Ethereum client URL")
	flag.StringVar(&port, "port", "3000", "HTTP port")
	flag.StringVar(&serviceName, "service", "KidsTube", "Platform service name")
	flag.StringVar(&baseDir, "basedir", ".", "Base directory for templates and static files")
	flag.StringVar(&veriqidServer, "veriqid-server", "http://127.0.0.1:8080", "Veriqid server URL for activity reporting")
	flag.Parse()

	if contractAddr == "" {
		log.Fatal("ERROR: -contract flag is required. Example: -contract 0x5FbDB2315678afecb367f032d93F642f64180aa3")
	}

	// Hash the service name (same as Phase 4 server)
	sha := sha256.Sum256([]byte(serviceName))
	serviceHash = hex.EncodeToString(sha[:])
	log.Printf("Service: %s → SHA-256: %s", serviceName, serviceHash[:16]+"...")

	// Initialize
	initDB()
	initContract()
	loadTemplates()

	// ─── Routes ───
	// Static files
	staticDir := filepath.Join(baseDir, "demo-platform", "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	// Public routes
	http.HandleFunc("/", handleLanding)
	http.HandleFunc("/signup", handleSignup)       // GET = form, POST = submit
	http.HandleFunc("/login", handleLogin)         // GET = form, POST = submit

	// Protected routes
	http.HandleFunc("/home", handleHome)
	http.HandleFunc("/profile", handleProfile)
	http.HandleFunc("/logout", handleLogout)

	// API
	http.HandleFunc("/api/status", handleAPIStatus)

	log.Printf("===========================================")
	log.Printf("  KidsTube Demo Platform")
	log.Printf("  http://localhost:%s", port)
	log.Printf("  Service: %s", serviceName)
	log.Printf("  Contract: %s", contractAddr)
	log.Printf("  Veriqid Server: %s", veriqidServer)
	log.Printf("===========================================")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
GOEOF
```

**What each section does:**

- **SessionStore** — In-memory cookie sessions (same pattern as Phase 4). Pseudonymous: stores only SPK (ASC pseudonym/nullifier) and username, no PII.
- **initDB()** — Creates `kidstube.db` (separate from `veriqid.db`) with `users` and `challenges` tables. The `contract_index` column enables revocation checking.
- **initContract()** — Connects to Ganache, loads Veriqid.sol bindings via `u2sso.NewVeriqid()`.
- **handleSignup / handleSignupSubmit** — The core ASC Verify() implementation. Generates challenge (message m), verifies ring membership proof (π) via `u2sso.RegistrationVerify()`, stores SPK (pseudonym/nullifier a) with UNIQUE constraint (Sybil-Resistance).
- **handleLogin / handleLoginSubmit** — Auth flow using `u2sso.AuthVerify()` (Boquila signature verification).
- **handleHome** — Protected page with on-chain revocation check via `checkRevocation()`.
- **checkRevocation()** — Looks up the user's `contract_index` from the database, then calls `contractInst.GetState(index)` on the smart contract. Returns `true` if the identity has been revoked (i.e., `active == false`).
- **reportPlatformActivity()** — Async POST to the Veriqid server (`POST /api/platform/activity`) so the parent dashboard can show platform-specific events.
- **findContractIndexBySPK()** — Fallback placeholder when extension doesn't provide contract_index. Returns -1 (the portable extension provides contract_index; the Phase 5 extension may not).

### 2.3 Verify the server compiles

```bash
go build ./cmd/demo-platform/
```

**Expected output:** No output = success. A binary named `demo-platform` appears.

**If it fails with import errors:**

Check that your `go.mod` module path matches the imports. The import path `github.com/patmekury/veriqid/pkg/u2sso` must match your module declaration. If your module is named differently, update the import in `main.go`.

```bash
head -1 go.mod
# Should show: module github.com/patmekury/veriqid
```

If your module path is different, replace `github.com/patmekury/veriqid` in the import with your actual module path.

```bash
rm -f demo-platform
```

---

## STEP 3: Create the HTML Templates

**Time: ~45 minutes**

### 3.1 Landing page (demo-platform/templates/landing.html)

This is the first thing judges see — a bright, kid-friendly video platform landing page with a prominent "Sign up with Veriqid" button.

```bash
cat > demo-platform/templates/landing.html << 'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>KidsTube — Safe Videos for Kids</title>
    <link rel="stylesheet" href="/static/css/kidstube.css">
</head>
<body class="landing-body">
    <header class="landing-header">
        <div class="logo">
            <span class="logo-icon">▶</span>
            <span class="logo-text">KidsTube</span>
        </div>
        <nav class="landing-nav">
            <a href="/login" class="btn btn-outline">Log In</a>
            <a href="/signup" class="btn btn-primary">Sign Up</a>
        </nav>
    </header>

    <main class="hero">
        <div class="hero-content">
            <h1>Safe, Fun Videos for Kids</h1>
            <p class="hero-subtitle">Age-verified. Privacy-first. Parent-approved.</p>
            <p class="hero-description">
                KidsTube uses <strong>Veriqid</strong> to verify your child's age without
                collecting any personal information. No ID uploads. No surveillance. Just safe fun.
            </p>
            <div class="hero-actions">
                <a href="/signup" class="btn btn-hero">
                    <span class="shield-icon">🛡️</span>
                    Sign up with Veriqid
                </a>
            </div>
            <p class="hero-note">
                Requires the <a href="#">Veriqid browser extension</a> — your child's identity stays on their device.
            </p>
        </div>
        <div class="hero-visual">
            <div class="mock-player">
                <div class="mock-screen">
                    <div class="play-button">▶</div>
                </div>
                <div class="mock-title">Amazing Science Experiments!</div>
                <div class="mock-channel">ScienceKids • 1.2M views</div>
            </div>
        </div>
    </main>

    <section class="features">
        <h2>Why KidsTube + Veriqid?</h2>
        <div class="feature-grid">
            <div class="feature-card">
                <div class="feature-icon">🔒</div>
                <h3>Zero Data Collection</h3>
                <p>We never see your child's name, age, or any personal info. Veriqid proves they're a verified kid — that's all we need.</p>
            </div>
            <div class="feature-card">
                <div class="feature-icon">👨‍👩‍👧</div>
                <h3>Parent Control</h3>
                <p>Parents verify once, control everywhere. Revoke access to all platforms with one click from the Veriqid dashboard.</p>
            </div>
            <div class="feature-card">
                <div class="feature-icon">🎭</div>
                <h3>Unlinkable Identity</h3>
                <p>Your child's KidsTube pseudonym can't be linked to their identity on other platforms. Each site sees a unique, service-specific key.</p>
            </div>
            <div class="feature-card">
                <div class="feature-icon">⚡</div>
                <h3>Instant Verification</h3>
                <p>Sign up in under 2 seconds. The browser extension handles everything automatically — no forms to fill.</p>
            </div>
        </div>
    </section>

    <section class="how-it-works">
        <h2>How It Works</h2>
        <div class="steps">
            <div class="step">
                <div class="step-number">1</div>
                <h3>Parent Verifies Child</h3>
                <p>Through the Veriqid dashboard — parent receives a 12-word portable key phrase.</p>
            </div>
            <div class="step-arrow">→</div>
            <div class="step">
                <div class="step-number">2</div>
                <h3>Install Extension</h3>
                <p>Child pastes the 12-word phrase into the Veriqid browser extension.</p>
            </div>
            <div class="step-arrow">→</div>
            <div class="step">
                <div class="step-number">3</div>
                <h3>Click "Sign Up"</h3>
                <p>Extension auto-fills proof. KidsTube verifies. Done in 2 seconds.</p>
            </div>
        </div>
    </section>

    <footer class="landing-footer">
        <p>KidsTube is a demo platform for the <strong>Veriqid</strong> hackathon project.</p>
        <p>No real videos. No real accounts. Just privacy-preserving age verification.</p>
    </footer>
</body>
</html>
HTMLEOF
```

### 3.2 Signup page (demo-platform/templates/signup.html)

This page contains the **hidden Veriqid fields** that the browser extension detects and auto-fills. The visible UI shows a simple username picker.

```bash
cat > demo-platform/templates/signup.html << 'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{if .IsLogin}}Log In{{else}}Sign Up{{end}} — KidsTube</title>
    <link rel="stylesheet" href="/static/css/kidstube.css">
</head>
<body class="auth-body">
    <header class="auth-header">
        <a href="/" class="logo">
            <span class="logo-icon">▶</span>
            <span class="logo-text">KidsTube</span>
        </a>
    </header>

    <main class="auth-main">
        <div class="auth-card">
            <div class="auth-card-header">
                <h1>{{if .IsLogin}}Welcome Back!{{else}}Join KidsTube{{end}}</h1>
                <p>{{if .IsLogin}}Log in with your Veriqid identity{{else}}Create your account with Veriqid — no personal info needed{{end}}</p>
            </div>

            <form action="{{if .IsLogin}}/login{{else}}/signup{{end}}" method="POST" id="veriqid-form">
                <!-- ═══════════════════════════════════════════════
                     VERIQID HIDDEN FIELDS
                     The browser extension (Phase 5 or portable) detects
                     these by ID. IDs must match exactly what content.js
                     looks for. These fields carry the ASC protocol data:
                     - challenge = message (m)
                     - service-name = hashed service identifier
                     - proof = ring membership proof (π)
                     - spk = pseudonym AND nullifier (a/nul)
                     - contract-index = on-chain ID index for revocation
                     ═══════════════════════════════════════════════ -->
                <input type="hidden" id="veriqid-challenge" name="veriqid-challenge" value="{{.Challenge}}">
                <input type="hidden" id="veriqid-service-name" name="veriqid-service-name" value="{{.ServiceName}}">
                <input type="hidden" id="veriqid-proof" name="veriqid-proof" value="">
                <input type="hidden" id="veriqid-spk" name="veriqid-spk" value="">
                <input type="hidden" id="veriqid-signature" name="veriqid-signature" value="">
                <input type="hidden" id="veriqid-decoy-size" name="veriqid-decoy-size" value="">
                <input type="hidden" id="veriqid-nullifier" name="veriqid-nullifier" value="">
                <input type="hidden" id="veriqid-contract-index" name="veriqid-contract-index" value="">

                {{if not .IsLogin}}
                <!-- Visible field: username (optional, pseudonymous) -->
                <div class="form-group">
                    <label for="username">Choose a display name</label>
                    <input type="text" id="username" name="username"
                           placeholder="e.g., CoolKid42, StarExplorer"
                           maxlength="30"
                           class="form-input">
                    <p class="form-hint">This is your KidsTube nickname — not your real name!</p>
                </div>
                {{end}}

                <div class="veriqid-status" id="veriqid-status">
                    <div class="status-icon">🛡️</div>
                    <div class="status-text">
                        <strong>Waiting for Veriqid extension...</strong>
                        <p>The browser extension will automatically verify your identity.</p>
                    </div>
                </div>

                <button type="submit" class="btn btn-primary btn-full" id="submit-btn" disabled>
                    {{if .IsLogin}}Log In with Veriqid{{else}}Create Account{{end}}
                </button>

                <div class="auth-footer">
                    {{if .IsLogin}}
                        <p>Don't have an account? <a href="/signup">Sign up</a></p>
                    {{else}}
                        <p>Already have an account? <a href="/login">Log in</a></p>
                    {{end}}
                </div>
            </form>
        </div>

        <div class="privacy-badge">
            <span class="badge-icon">🔒</span>
            <span>KidsTube never sees your personal information</span>
        </div>
    </main>

    <script src="/static/js/kidstube.js"></script>
</body>
</html>
HTMLEOF
```

**Key points about the hidden fields:**

- The `id` attributes (`veriqid-challenge`, `veriqid-service-name`, `veriqid-proof`, `veriqid-spk`, `veriqid-contract-index`, etc.) are what the content script scans for via `document.getElementById()`.
- The extension reads the challenge and service name from the hidden fields, calls the bridge API to generate a proof, and writes the proof/spk/signature back into the hidden fields.
- The `veriqid-contract-index` field is filled by the portable extension — this enables KidsTube to perform on-chain revocation checks.
- The submit button starts `disabled` — the extension enables it after filling in the proof.
- The form POSTs to the same URL (`/signup` or `/login`) — the Go handler routes by method.

### 3.3 Home page (demo-platform/templates/home.html)

The protected content page — shows mock video thumbnails after successful signup/login.

```bash
cat > demo-platform/templates/home.html << 'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>KidsTube — Home</title>
    <link rel="stylesheet" href="/static/css/kidstube.css">
</head>
<body class="app-body">
    <header class="app-header">
        <div class="logo">
            <span class="logo-icon">▶</span>
            <span class="logo-text">KidsTube</span>
        </div>
        <div class="header-right">
            <span class="user-badge">
                <span class="badge-avatar">{{slice .Username 0 1}}</span>
                {{.Username}}
            </span>
            <a href="/profile" class="btn btn-small btn-outline">Profile</a>
            <a href="/logout" class="btn btn-small btn-ghost">Log Out</a>
        </div>
    </header>

    <main class="content-main">
        <section class="welcome-banner">
            <h1>Welcome, {{.Username}}! 🎉</h1>
            <p>Here are today's top videos — just for verified kids.</p>
            <div class="verified-badge">
                <span>🛡️</span> Verified with Veriqid — no personal data shared
            </div>
        </section>

        <section class="video-grid">
            <h2>Recommended For You</h2>
            <div class="videos">
                <div class="video-card">
                    <div class="video-thumbnail" style="background: linear-gradient(135deg, #FF6B6B, #FFA07A);">
                        <div class="play-overlay">▶</div>
                        <span class="duration">5:23</span>
                    </div>
                    <div class="video-info">
                        <h3>Amazing Space Facts!</h3>
                        <p class="channel">SpaceKids • 890K views • 2 days ago</p>
                    </div>
                </div>
                <div class="video-card">
                    <div class="video-thumbnail" style="background: linear-gradient(135deg, #4ECDC4, #2ECC71);">
                        <div class="play-overlay">▶</div>
                        <span class="duration">8:15</span>
                    </div>
                    <div class="video-info">
                        <h3>Build a Volcano at Home</h3>
                        <p class="channel">ScienceTime • 1.2M views • 1 week ago</p>
                    </div>
                </div>
                <div class="video-card">
                    <div class="video-thumbnail" style="background: linear-gradient(135deg, #A78BFA, #818CF8);">
                        <div class="play-overlay">▶</div>
                        <span class="duration">12:07</span>
                    </div>
                    <div class="video-info">
                        <h3>Learn to Code: First Game!</h3>
                        <p class="channel">CodeKids • 650K views • 3 days ago</p>
                    </div>
                </div>
                <div class="video-card">
                    <div class="video-thumbnail" style="background: linear-gradient(135deg, #F59E0B, #EF4444);">
                        <div class="play-overlay">▶</div>
                        <span class="duration">6:41</span>
                    </div>
                    <div class="video-info">
                        <h3>World's Coolest Animals</h3>
                        <p class="channel">NatureWow • 2.1M views • 5 days ago</p>
                    </div>
                </div>
                <div class="video-card">
                    <div class="video-thumbnail" style="background: linear-gradient(135deg, #EC4899, #F472B6);">
                        <div class="play-overlay">▶</div>
                        <span class="duration">4:55</span>
                    </div>
                    <div class="video-info">
                        <h3>Easy Drawing Tutorial</h3>
                        <p class="channel">ArtFun • 430K views • 1 day ago</p>
                    </div>
                </div>
                <div class="video-card">
                    <div class="video-thumbnail" style="background: linear-gradient(135deg, #06B6D4, #3B82F6);">
                        <div class="play-overlay">▶</div>
                        <span class="duration">10:30</span>
                    </div>
                    <div class="video-info">
                        <h3>Ocean Mysteries Explained</h3>
                        <p class="channel">DeepBlue • 780K views • 4 days ago</p>
                    </div>
                </div>
            </div>
        </section>
    </main>

    <footer class="app-footer">
        <p>🛡️ KidsTube verified by Veriqid — <a href="/profile">View your pseudonymous profile</a></p>
    </footer>
</body>
</html>
HTMLEOF
```

### 3.4 Profile page (demo-platform/templates/profile.html)

Shows the pseudonymous account info — emphasizes that KidsTube knows nothing about the real child.

```bash
cat > demo-platform/templates/profile.html << 'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Profile — KidsTube</title>
    <link rel="stylesheet" href="/static/css/kidstube.css">
</head>
<body class="app-body">
    <header class="app-header">
        <a href="/home" class="logo">
            <span class="logo-icon">▶</span>
            <span class="logo-text">KidsTube</span>
        </a>
        <div class="header-right">
            <a href="/home" class="btn btn-small btn-outline">← Back to Videos</a>
            <a href="/logout" class="btn btn-small btn-ghost">Log Out</a>
        </div>
    </header>

    <main class="profile-main">
        <div class="profile-card">
            <div class="profile-avatar">
                <span>{{slice .Username 0 1}}</span>
            </div>
            <h1>{{.Username}}</h1>
            <div class="profile-verified">
                <span>🛡️</span> Verified with Veriqid
            </div>

            <div class="profile-details">
                <div class="detail-row">
                    <span class="detail-label">Display Name</span>
                    <span class="detail-value">{{.Username}}</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Pseudonymous ID (SPK)</span>
                    <span class="detail-value mono">{{.SPKShort}}</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Platform</span>
                    <span class="detail-value">{{.ServiceName}}</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Session Started</span>
                    <span class="detail-value">{{.CreatedAt}}</span>
                </div>
            </div>

            <div class="privacy-info">
                <h3>🔒 What KidsTube Knows About You</h3>
                <div class="privacy-grid">
                    <div class="privacy-item yes">
                        <span class="check">✓</span>
                        You are a verified child
                    </div>
                    <div class="privacy-item no">
                        <span class="cross">✗</span>
                        Your real name
                    </div>
                    <div class="privacy-item no">
                        <span class="cross">✗</span>
                        Your exact age
                    </div>
                    <div class="privacy-item no">
                        <span class="cross">✗</span>
                        Your parent's info
                    </div>
                    <div class="privacy-item no">
                        <span class="cross">✗</span>
                        Your accounts on other platforms
                    </div>
                    <div class="privacy-item no">
                        <span class="cross">✗</span>
                        Any government ID
                    </div>
                </div>
                <p class="privacy-note">
                    Your pseudonymous ID (<code>{{.SPKShort}}</code>) is unique to KidsTube
                    and cannot be linked to your identity on any other platform.
                    This is the ASC <strong>Unlinkability</strong> property in action.
                </p>
            </div>
        </div>
    </main>
</body>
</html>
HTMLEOF
```

### 3.5 Revoked page (demo-platform/templates/revoked.html)

Shown when a parent has revoked the child's identity from the Phase 6 dashboard.

```bash
cat > demo-platform/templates/revoked.html << 'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Access Revoked — KidsTube</title>
    <link rel="stylesheet" href="/static/css/kidstube.css">
</head>
<body class="revoked-body">
    <div class="revoked-container">
        <div class="revoked-icon">🔒</div>
        <h1>Access Revoked</h1>
        <p>Your Veriqid identity has been revoked by your parent or guardian.</p>
        <p>This means your account on KidsTube (and all other platforms) is no longer active.</p>
        <div class="revoked-actions">
            <p>If you think this is a mistake, please talk to your parent.</p>
            <a href="/" class="btn btn-outline">Return to KidsTube</a>
        </div>
    </div>
</body>
</html>
HTMLEOF
```

### 3.6 Verify all templates were created

```bash
ls -la demo-platform/templates/
```

**Expected output:**

```
landing.html
signup.html
home.html
profile.html
revoked.html
```

---

## STEP 4: Create the CSS Stylesheet

**Time: ~30 minutes**

### 4.1 Create demo-platform/static/css/kidstube.css

The KidsTube design is intentionally **different** from the Veriqid teal/lime design system — it uses bright, warm, kid-friendly colors to look like a real consumer platform.

```bash
cat > demo-platform/static/css/kidstube.css << 'CSSEOF'
/* ═══════════════════════════════════════════
   KidsTube Design System
   Bright, warm, kid-friendly palette
   Intentionally distinct from Veriqid branding
   ═══════════════════════════════════════════ */

:root {
    /* Primary */
    --kt-red:           #FF4444;
    --kt-red-dark:      #CC3333;
    --kt-red-light:     #FF6666;

    /* Accents */
    --kt-orange:        #FF8C00;
    --kt-yellow:        #FFD700;
    --kt-green:         #4CAF50;
    --kt-blue:          #2196F3;
    --kt-purple:        #9C27B0;

    /* Neutrals */
    --kt-white:         #FFFFFF;
    --kt-bg:            #FAFAFA;
    --kt-gray-100:      #F5F5F5;
    --kt-gray-200:      #EEEEEE;
    --kt-gray-300:      #E0E0E0;
    --kt-gray-400:      #BDBDBD;
    --kt-gray-500:      #9E9E9E;
    --kt-gray-600:      #757575;
    --kt-gray-700:      #616161;
    --kt-gray-800:      #424242;
    --kt-gray-900:      #212121;

    /* Typography */
    --kt-font:          'Segoe UI', -apple-system, BlinkMacSystemFont, sans-serif;
    --kt-font-round:    'Nunito', 'Segoe UI', sans-serif;

    /* Shadows */
    --kt-shadow-sm:     0 1px 3px rgba(0,0,0,0.08);
    --kt-shadow-md:     0 4px 12px rgba(0,0,0,0.1);
    --kt-shadow-lg:     0 8px 24px rgba(0,0,0,0.12);

    /* Radius */
    --kt-radius-sm:     8px;
    --kt-radius-md:     12px;
    --kt-radius-lg:     16px;
    --kt-radius-xl:     24px;
}

/* ─── Reset ─── */
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: var(--kt-font); color: var(--kt-gray-900); background: var(--kt-bg); }
a { color: var(--kt-blue); text-decoration: none; }
a:hover { text-decoration: underline; }

/* ─── Buttons ─── */
.btn {
    display: inline-flex; align-items: center; gap: 8px;
    padding: 10px 20px; border-radius: var(--kt-radius-sm);
    font-size: 14px; font-weight: 600; cursor: pointer;
    border: 2px solid transparent; transition: all 0.2s;
    text-decoration: none;
}
.btn-primary {
    background: var(--kt-red); color: white; border-color: var(--kt-red);
}
.btn-primary:hover { background: var(--kt-red-dark); border-color: var(--kt-red-dark); text-decoration: none; }
.btn-primary:disabled { background: var(--kt-gray-400); border-color: var(--kt-gray-400); cursor: not-allowed; }
.btn-outline {
    background: transparent; color: var(--kt-gray-800); border-color: var(--kt-gray-300);
}
.btn-outline:hover { border-color: var(--kt-gray-500); text-decoration: none; }
.btn-ghost { background: transparent; color: var(--kt-gray-600); }
.btn-ghost:hover { color: var(--kt-gray-900); text-decoration: none; }
.btn-hero {
    background: var(--kt-red); color: white; border-color: var(--kt-red);
    padding: 16px 32px; font-size: 18px; border-radius: var(--kt-radius-md);
}
.btn-hero:hover { background: var(--kt-red-dark); transform: translateY(-2px); box-shadow: var(--kt-shadow-md); text-decoration: none; }
.btn-small { padding: 6px 12px; font-size: 13px; }
.btn-full { width: 100%; justify-content: center; padding: 14px; font-size: 16px; }

/* ─── Logo ─── */
.logo { display: flex; align-items: center; gap: 8px; text-decoration: none; color: inherit; }
.logo-icon {
    display: flex; align-items: center; justify-content: center;
    width: 36px; height: 36px; background: var(--kt-red); color: white;
    border-radius: 8px; font-size: 16px;
}
.logo-text { font-size: 22px; font-weight: 800; color: var(--kt-gray-900); }

/* ═══════════════════════════════════════════
   LANDING PAGE
   ═══════════════════════════════════════════ */

.landing-body { background: white; }

.landing-header {
    display: flex; justify-content: space-between; align-items: center;
    padding: 16px 40px; border-bottom: 1px solid var(--kt-gray-200);
}
.landing-nav { display: flex; gap: 12px; }

.hero {
    display: flex; align-items: center; justify-content: space-between;
    max-width: 1200px; margin: 0 auto; padding: 80px 40px;
    gap: 60px;
}
.hero-content { flex: 1; max-width: 560px; }
.hero-content h1 { font-size: 48px; font-weight: 800; line-height: 1.1; margin-bottom: 16px; }
.hero-subtitle { font-size: 20px; color: var(--kt-gray-600); margin-bottom: 16px; }
.hero-description { font-size: 16px; color: var(--kt-gray-700); line-height: 1.6; margin-bottom: 32px; }
.hero-actions { margin-bottom: 16px; }
.hero-note { font-size: 13px; color: var(--kt-gray-500); }
.shield-icon { font-size: 20px; }

.hero-visual { flex: 0 0 380px; }
.mock-player { background: var(--kt-gray-100); border-radius: var(--kt-radius-lg); overflow: hidden; box-shadow: var(--kt-shadow-lg); }
.mock-screen {
    height: 220px; background: linear-gradient(135deg, #667eea, #764ba2);
    display: flex; align-items: center; justify-content: center;
}
.play-button {
    width: 60px; height: 60px; background: rgba(255,255,255,0.9);
    border-radius: 50%; display: flex; align-items: center; justify-content: center;
    font-size: 24px; color: #764ba2;
}
.mock-title { padding: 16px 16px 4px; font-weight: 700; font-size: 15px; }
.mock-channel { padding: 0 16px 16px; font-size: 13px; color: var(--kt-gray-500); }

/* Features */
.features {
    background: var(--kt-gray-100); padding: 80px 40px; text-align: center;
}
.features h2 { font-size: 32px; margin-bottom: 48px; }
.feature-grid {
    display: grid; grid-template-columns: repeat(4, 1fr); gap: 24px;
    max-width: 1200px; margin: 0 auto;
}
.feature-card {
    background: white; padding: 32px 24px; border-radius: var(--kt-radius-md);
    box-shadow: var(--kt-shadow-sm); text-align: center;
}
.feature-icon { font-size: 40px; margin-bottom: 16px; }
.feature-card h3 { font-size: 18px; margin-bottom: 8px; }
.feature-card p { font-size: 14px; color: var(--kt-gray-600); line-height: 1.5; }

/* How it works */
.how-it-works { padding: 80px 40px; text-align: center; max-width: 1000px; margin: 0 auto; }
.how-it-works h2 { font-size: 32px; margin-bottom: 48px; }
.steps { display: flex; align-items: flex-start; justify-content: center; gap: 16px; }
.step {
    flex: 1; max-width: 250px; text-align: center;
}
.step-number {
    display: inline-flex; align-items: center; justify-content: center;
    width: 48px; height: 48px; background: var(--kt-red); color: white;
    border-radius: 50%; font-size: 20px; font-weight: 700; margin-bottom: 16px;
}
.step h3 { font-size: 16px; margin-bottom: 8px; }
.step p { font-size: 14px; color: var(--kt-gray-600); }
.step-arrow { font-size: 24px; color: var(--kt-gray-400); padding-top: 12px; }

/* Footer */
.landing-footer {
    padding: 32px 40px; text-align: center;
    border-top: 1px solid var(--kt-gray-200);
    font-size: 13px; color: var(--kt-gray-500);
}
.landing-footer p { margin: 4px 0; }

/* ═══════════════════════════════════════════
   AUTH PAGE (Signup / Login)
   ═══════════════════════════════════════════ */

.auth-body { background: var(--kt-gray-100); min-height: 100vh; }
.auth-header { padding: 16px 40px; }
.auth-main {
    display: flex; flex-direction: column; align-items: center;
    justify-content: center; min-height: calc(100vh - 68px);
    padding: 40px 20px;
}
.auth-card {
    background: white; border-radius: var(--kt-radius-lg);
    box-shadow: var(--kt-shadow-md); padding: 40px; max-width: 460px; width: 100%;
}
.auth-card-header { text-align: center; margin-bottom: 32px; }
.auth-card-header h1 { font-size: 24px; margin-bottom: 8px; }
.auth-card-header p { font-size: 14px; color: var(--kt-gray-600); }

.form-group { margin-bottom: 20px; }
.form-group label { display: block; font-size: 14px; font-weight: 600; margin-bottom: 6px; }
.form-input {
    width: 100%; padding: 12px 16px; border: 2px solid var(--kt-gray-300);
    border-radius: var(--kt-radius-sm); font-size: 15px; transition: border-color 0.2s;
}
.form-input:focus { outline: none; border-color: var(--kt-blue); }
.form-hint { font-size: 12px; color: var(--kt-gray-500); margin-top: 4px; }

.veriqid-status {
    display: flex; align-items: center; gap: 12px;
    padding: 16px; background: #FFF8E1; border: 1px solid #FFE082;
    border-radius: var(--kt-radius-sm); margin-bottom: 20px;
}
.veriqid-status.ready {
    background: #E8F5E9; border-color: #A5D6A7;
}
.status-icon { font-size: 24px; }
.status-text strong { display: block; font-size: 14px; }
.status-text p { font-size: 12px; color: var(--kt-gray-600); margin-top: 2px; }

.auth-footer { text-align: center; margin-top: 20px; font-size: 14px; color: var(--kt-gray-600); }

.privacy-badge {
    display: flex; align-items: center; gap: 8px;
    margin-top: 24px; font-size: 13px; color: var(--kt-gray-500);
}
.badge-icon { font-size: 16px; }

/* ═══════════════════════════════════════════
   APP PAGES (Home, Profile)
   ═══════════════════════════════════════════ */

.app-body { background: var(--kt-bg); }
.app-header {
    display: flex; justify-content: space-between; align-items: center;
    padding: 12px 40px; background: white; border-bottom: 1px solid var(--kt-gray-200);
    box-shadow: var(--kt-shadow-sm);
}
.header-right { display: flex; align-items: center; gap: 12px; }
.user-badge {
    display: flex; align-items: center; gap: 8px;
    font-size: 14px; font-weight: 600;
}
.badge-avatar {
    display: flex; align-items: center; justify-content: center;
    width: 28px; height: 28px; background: var(--kt-red);
    color: white; border-radius: 50%; font-size: 13px; font-weight: 700;
}

.content-main { max-width: 1200px; margin: 0 auto; padding: 32px 40px; }

.welcome-banner {
    background: linear-gradient(135deg, #FF6B6B, #FF8E53);
    color: white; padding: 32px; border-radius: var(--kt-radius-lg);
    margin-bottom: 40px;
}
.welcome-banner h1 { font-size: 28px; margin-bottom: 8px; }
.welcome-banner p { font-size: 16px; opacity: 0.9; }
.verified-badge {
    display: inline-flex; align-items: center; gap: 6px;
    margin-top: 12px; padding: 6px 12px; background: rgba(255,255,255,0.2);
    border-radius: 20px; font-size: 13px;
}

/* Video grid */
.video-grid h2 { font-size: 22px; margin-bottom: 20px; }
.videos {
    display: grid; grid-template-columns: repeat(3, 1fr); gap: 24px;
}
.video-card {
    background: white; border-radius: var(--kt-radius-md);
    overflow: hidden; box-shadow: var(--kt-shadow-sm);
    transition: transform 0.2s, box-shadow 0.2s; cursor: pointer;
}
.video-card:hover { transform: translateY(-4px); box-shadow: var(--kt-shadow-md); }
.video-thumbnail {
    height: 180px; position: relative;
    display: flex; align-items: center; justify-content: center;
}
.play-overlay {
    width: 48px; height: 48px; background: rgba(0,0,0,0.6);
    color: white; border-radius: 50%; display: flex;
    align-items: center; justify-content: center;
    font-size: 18px; opacity: 0; transition: opacity 0.2s;
}
.video-card:hover .play-overlay { opacity: 1; }
.duration {
    position: absolute; bottom: 8px; right: 8px;
    background: rgba(0,0,0,0.8); color: white;
    padding: 2px 6px; border-radius: 4px; font-size: 12px;
}
.video-info { padding: 12px 16px; }
.video-info h3 { font-size: 15px; margin-bottom: 4px; }
.channel { font-size: 13px; color: var(--kt-gray-500); }

.app-footer {
    text-align: center; padding: 24px; font-size: 13px; color: var(--kt-gray-500);
    border-top: 1px solid var(--kt-gray-200); margin-top: 60px;
}

/* ═══════════════════════════════════════════
   PROFILE PAGE
   ═══════════════════════════════════════════ */

.profile-main {
    display: flex; justify-content: center; padding: 40px 20px;
}
.profile-card {
    background: white; border-radius: var(--kt-radius-lg);
    box-shadow: var(--kt-shadow-md); padding: 40px;
    max-width: 560px; width: 100%; text-align: center;
}
.profile-avatar {
    display: inline-flex; align-items: center; justify-content: center;
    width: 80px; height: 80px; background: var(--kt-red); color: white;
    border-radius: 50%; font-size: 36px; font-weight: 700; margin-bottom: 16px;
}
.profile-card h1 { font-size: 24px; margin-bottom: 8px; }
.profile-verified {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 6px 16px; background: #E8F5E9; color: #2E7D32;
    border-radius: 20px; font-size: 13px; font-weight: 600; margin-bottom: 32px;
}

.profile-details { text-align: left; margin-bottom: 32px; }
.detail-row {
    display: flex; justify-content: space-between; align-items: center;
    padding: 12px 0; border-bottom: 1px solid var(--kt-gray-200);
}
.detail-label { font-size: 14px; color: var(--kt-gray-600); }
.detail-value { font-size: 14px; font-weight: 600; }
.detail-value.mono { font-family: 'Courier New', monospace; font-size: 13px; }

.privacy-info {
    background: var(--kt-gray-100); border-radius: var(--kt-radius-md);
    padding: 24px; text-align: left;
}
.privacy-info h3 { font-size: 16px; margin-bottom: 16px; }
.privacy-grid { display: grid; gap: 8px; margin-bottom: 16px; }
.privacy-item {
    display: flex; align-items: center; gap: 10px;
    font-size: 14px; padding: 6px 0;
}
.privacy-item.yes { color: var(--kt-green); }
.privacy-item.no { color: var(--kt-gray-500); }
.check { font-weight: 700; }
.cross { font-weight: 700; }
.privacy-note { font-size: 13px; color: var(--kt-gray-600); line-height: 1.5; }
.privacy-note code {
    background: var(--kt-gray-200); padding: 2px 6px; border-radius: 4px;
    font-size: 12px;
}

/* ═══════════════════════════════════════════
   REVOKED PAGE
   ═══════════════════════════════════════════ */

.revoked-body {
    background: var(--kt-gray-100); min-height: 100vh;
    display: flex; align-items: center; justify-content: center;
}
.revoked-container {
    background: white; border-radius: var(--kt-radius-lg);
    box-shadow: var(--kt-shadow-md); padding: 60px 40px;
    max-width: 480px; text-align: center;
}
.revoked-icon { font-size: 64px; margin-bottom: 24px; }
.revoked-container h1 { font-size: 28px; margin-bottom: 12px; color: var(--kt-red); }
.revoked-container p { font-size: 15px; color: var(--kt-gray-600); line-height: 1.6; margin-bottom: 8px; }
.revoked-actions { margin-top: 32px; }
.revoked-actions p { font-size: 14px; margin-bottom: 16px; }

/* ─── Responsive ─── */
@media (max-width: 900px) {
    .hero { flex-direction: column; padding: 40px 20px; text-align: center; }
    .hero-visual { flex: none; width: 100%; max-width: 380px; }
    .feature-grid { grid-template-columns: repeat(2, 1fr); }
    .videos { grid-template-columns: repeat(2, 1fr); }
    .steps { flex-direction: column; align-items: center; }
    .step-arrow { transform: rotate(90deg); }
}
@media (max-width: 600px) {
    .feature-grid { grid-template-columns: 1fr; }
    .videos { grid-template-columns: 1fr; }
    .landing-header { padding: 12px 16px; }
    .content-main { padding: 20px 16px; }
}
CSSEOF
```

---

## STEP 5: Create the Client-Side JavaScript

**Time: ~20 minutes**

### 5.1 Create demo-platform/static/js/kidstube.js

This script monitors the hidden Veriqid fields — when the extension fills them in, it enables the submit button and updates the status indicator.

```bash
cat > demo-platform/static/js/kidstube.js << 'JSEOF'
/**
 * KidsTube client-side logic
 * Monitors Veriqid hidden fields for extension auto-fill
 */
(function() {
    'use strict';

    const proofField     = document.getElementById('veriqid-proof');
    const spkField       = document.getElementById('veriqid-spk');
    const signatureField = document.getElementById('veriqid-signature');
    const submitBtn      = document.getElementById('submit-btn');
    const statusDiv      = document.getElementById('veriqid-status');

    if (!proofField || !submitBtn) return; // Not on auth page

    // Check if this is a login form (has signature field) or signup (has proof field)
    const isLogin = window.location.pathname.includes('login');

    /**
     * Poll hidden fields for extension auto-fill.
     * The content script (Phase 5 or portable extension) writes to these fields directly.
     */
    function checkExtensionFill() {
        const filled = isLogin
            ? (signatureField && signatureField.value.length > 0 && spkField.value.length > 0)
            : (proofField.value.length > 0 && spkField.value.length > 0);

        if (filled) {
            // Extension has filled in the proof!
            submitBtn.disabled = false;

            if (statusDiv) {
                statusDiv.classList.add('ready');
                statusDiv.querySelector('.status-icon').textContent = '✅';
                statusDiv.querySelector('strong').textContent = 'Veriqid identity verified!';
                statusDiv.querySelector('p').textContent = 'Your proof has been generated. Click the button below to continue.';
            }

            console.log('[KidsTube] Veriqid proof detected — submit enabled');
            return; // Stop polling
        }

        // Keep polling
        setTimeout(checkExtensionFill, 500);
    }

    // Also listen for input events (in case the extension triggers them)
    [proofField, spkField, signatureField].forEach(field => {
        if (field) {
            field.addEventListener('input', checkExtensionFill);
            // MutationObserver for value changes via JS (extension uses .value = ...)
            const observer = new MutationObserver(checkExtensionFill);
            observer.observe(field, { attributes: true, attributeFilter: ['value'] });
        }
    });

    // Start polling
    setTimeout(checkExtensionFill, 1000);

    // Allow manual submission for testing (if no extension installed)
    // After 10 seconds, show a manual mode hint
    setTimeout(function() {
        if (submitBtn.disabled && statusDiv) {
            const hint = document.createElement('p');
            hint.style.cssText = 'font-size:12px; color:#999; margin-top:8px;';
            hint.textContent = 'Extension not detected. For testing: use the bridge CLI to generate proof values and paste them into the browser console.';
            statusDiv.appendChild(hint);
        }
    }, 10000);

})();
JSEOF
```

---

## STEP 6: Build, Run, and Test

**Time: ~30 minutes**

### 6.1 Make sure all dependencies are running

You need **4 terminals** for the full demo:

| Terminal | Process | Port |
|----------|---------|------|
| 1 | Ganache | 7545 |
| 2 | Bridge server | 9090 |
| 3 | Veriqid server (Phase 4/6) | 8080 |
| 4 | KidsTube server (Phase 7) | 3000 |

### 6.2 Build and start KidsTube

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"

go build -o kidstube-server ./cmd/demo-platform/

./kidstube-server \
  -contract 0x<CONTRACT_ADDRESS> \
  -client http://127.0.0.1:7545 \
  -port 3000 \
  -service "KidsTube" \
  -basedir . \
  -veriqid-server http://127.0.0.1:8080
```

**Expected output:**

```
2026/03/12 ... Service: KidsTube → SHA-256: a1b2c3d4...
2026/03/12 ... Connected to contract at 0x... — 2 identities registered
2026/03/12 ... ===========================================
2026/03/12 ...   KidsTube Demo Platform
2026/03/12 ...   http://localhost:3000
2026/03/12 ...   Service: KidsTube
2026/03/12 ...   Contract: 0x...
2026/03/12 ...   Veriqid Server: http://127.0.0.1:8080
2026/03/12 ... ===========================================
```

### 6.3 Test the landing page

Open your browser and go to:

```
http://localhost:3000
```

**What you should see:** A bright, kid-friendly landing page with:
- KidsTube logo (red play button + text)
- Hero section with "Safe, Fun Videos for Kids" heading
- "Sign up with Veriqid" hero button
- Feature cards explaining privacy, parent control, unlinkability, instant verification
- "How it works" steps (now updated to reference 12-word portable key)

### 6.4 Test the signup flow

1. Click **"Sign up with Veriqid"** (or the "Sign Up" button in the nav)
2. You should see the signup card with:
   - A username input field
   - A yellow "Waiting for Veriqid extension..." status indicator
   - A disabled "Create Account" button
3. If the **browser extension** (Phase 5 or portable) is installed and the bridge is running:
   - The extension detects the hidden `veriqid-challenge` and `veriqid-service-name` fields
   - It calls the bridge API to generate a proof (ASC Prove)
   - It writes the proof, SPK, decoy size, and contract-index into the hidden fields
   - The status indicator turns green: "Veriqid identity verified!"
   - The submit button becomes active
4. Type a username (e.g., "CoolKid42") and click **"Create Account"**

### 6.5 Test without the extension (manual CLI fallback)

If the extension isn't installed yet, you can test the flow manually using the bridge or CLI:

```bash
# In a new terminal:
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"

# 1. Visit http://localhost:3000/signup in your browser
# 2. Open browser dev tools (F12) → Console
# 3. Read the challenge and service name from the hidden fields:
#    document.getElementById('veriqid-challenge').value
#    document.getElementById('veriqid-service-name').value

# 4. Generate a proof via CLI:
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -command register \
  -keypath ./key1 \
  -sname <SERVICE_NAME_HEX> \
  -challenge <CHALLENGE_HEX>

# 5. Copy the output values into the browser console:
#    document.getElementById('veriqid-proof').value = '<PROOF_HEX>'
#    document.getElementById('veriqid-spk').value = '<SPK_HEX>'
#    document.getElementById('veriqid-decoy-size').value = '<N>'
#    document.getElementById('veriqid-contract-index').value = '<INDEX>'
#    document.getElementById('submit-btn').disabled = false

# 6. Click "Create Account"
```

### 6.6 Verify the home page

After successful signup, you should be redirected to `localhost:3000/home` and see:
- A colorful welcome banner: "Welcome, CoolKid42! 🎉"
- "Verified with Veriqid — no personal data shared" badge
- A grid of mock video thumbnails (colorful gradient cards with play buttons)

### 6.7 Verify the profile page

Click "Profile" in the header. You should see:
- Username and avatar initial
- "Verified with Veriqid" badge
- Pseudonymous ID (truncated SPK hex) — labeled "Pseudonymous ID (SPK)"
- Privacy info panel showing what KidsTube knows (only "You are a verified child") and what it DOESN'T know (real name, exact age, parent info, other platform accounts, government ID)
- Note about ASC Unlinkability property

### 6.8 Test the login flow

1. Click "Log Out"
2. Go to `localhost:3000/login`
3. The extension auto-fills an auth proof (or use the CLI with `-command auth`)
4. You should be redirected back to `/home`

---

## STEP 7: End-to-End Demo Script (All Phases Together)

**Time: ~30 minutes to rehearse**

This is the **hackathon demo script** — the sequence that shows all phases working together.

### 7.1 Prerequisites (all running)

```
Terminal 1: ganache --port 7545
Terminal 2: ./bridge-server -contract 0x<ADDR> -client http://127.0.0.1:7545
Terminal 3: go run ./cmd/veriqid-server -contract 0x<ADDR> -service "KidsTube" -port 8080 -master-secret "my-hackathon-secret"
Terminal 4: ./kidstube-server -contract 0x<ADDR> -client http://127.0.0.1:7545 -port 3000 -service "KidsTube" -veriqid-server http://127.0.0.1:8080
```

At least 2 identities must be registered on the contract.

### 7.2 Demo sequence

| Step | Action | What Judges See |
|------|--------|-----------------|
| 1 | Open `localhost:8080/parent` | Parent dashboard — "Create your Veriqid account" |
| 2 | Create parent account (email + password) | Professional onboarding flow |
| 3 | Add child "Alex", age bracket "Under 13" | Simple form — no crypto visible |
| 4 | **Parent receives 12-word mnemonic phrase** | Grid of 12 kid-friendly words — one-time display |
| 5 | Child pastes mnemonic into portable extension | Extension stores MSK derived from phrase |
| 6 | Open `localhost:3000` in a new tab | KidsTube landing page — bright, kid-friendly |
| 7 | Click "Sign up with Veriqid" | Signup form — extension auto-fills proof in ~1 second |
| 8 | Type username "AlexPlays", click "Create Account" | Redirected to video grid — "Welcome, AlexPlays!" |
| 9 | Click "Profile" | Shows pseudonymous ID, what KidsTube does/doesn't know |
| 10 | Switch back to parent dashboard tab | Activity log shows "Alex registered on KidsTube" |
| 11 | Click "Revoke" on Alex's identity card | Confirmation dialog — enter password — click "Revoke" |
| 12 | Switch to KidsTube tab, refresh | "Access Revoked — Your Veriqid identity has been revoked by your parent" |

### 7.3 Key talking points during the demo

**At Step 4 (mnemonic display):**
> "The parent just received a 12-word phrase. This encodes the child's master secret key. The parent gives this phrase to the child — no private key management, no hardware tokens. Just 12 words a kid can type."

**At Step 6 (KidsTube landing):**
> "This is a completely separate platform. It has its own server, its own database, its own branding. It knows nothing about Veriqid's internals."

**At Step 8 (successful signup):**
> "That signup took under 2 seconds. The browser extension handled everything — it ran the ASC Prove algorithm locally and sent the proof to KidsTube. KidsTube verified the proof against the on-chain anonymity set but learned absolutely nothing about the child's real identity."

**At Step 9 (profile page):**
> "Look at what KidsTube knows vs. what it doesn't. No real name, no exact age, no parent info, no government ID, and critically — no way to link this account to the child's accounts on any other platform. That's the ASC Unlinkability property."

**At Step 12 (revocation):**
> "The parent just revoked the master identity on-chain. KidsTube checked the contract and detected it. This kills access everywhere — KidsTube, any future platform, everything — with one click. And the parent never touched a blockchain, never saw a private key, never ran a CLI command."

---

## STEP 8: Phase 7 Completion Checklist

Run through this checklist to confirm everything is working:

```
[ ] demo-platform/ directory created with static/ and templates/ subdirectories
[ ] cmd/demo-platform/main.go compiles (go build ./cmd/demo-platform/)
[ ] Code uses u2sso.NewVeriqid() (NOT u2sso.NewU2sso())
[ ] kidstube.db created on first run (separate from veriqid.db) with contract_index column
[ ] Landing page loads at localhost:3000 (kid-friendly design, NOT Veriqid branding)
[ ] Signup page has hidden veriqid-* fields including veriqid-contract-index (inspect with browser dev tools)
[ ] Browser extension (Phase 5 or portable) detects the signup form and auto-fills proof
[ ] Signup verification succeeds (server log: "Registration proof VALID")
[ ] Home page shows mock video grid behind auth wall
[ ] Profile page shows pseudonymous ID and privacy info panel
[ ] Login flow works (extension auto-fills auth proof)
[ ] Logout clears session, redirects to landing
[ ] Revocation detection works (checkRevocation queries contract via stored contract_index)
[ ] Platform activity reported to Veriqid server (server log: "Platform activity reported: registered")
[ ] KidsTube runs on port 3000 while Veriqid server runs on port 8080
[ ] End-to-end demo script works (parent dashboard → mnemonic → portable extension → KidsTube signup → revoke → blocked)
```

---

## What Each New File Does (Reference)

### cmd/demo-platform/main.go (~690 lines)
The KidsTube Go server. Configurable via flags (`-contract`, `-client`, `-port`, `-service`, `-basedir`, `-veriqid-server`). Uses `pkg/u2sso` for challenge generation (`CreateChallenge`), registration verification (`RegistrationVerify` — implements ASC Verify), and auth verification (`AuthVerify`). Contract binding: `*u2sso.Veriqid` via `u2sso.NewVeriqid()`. SQLite database (`kidstube.db`) stores users (with `contract_index` column) and challenges. In-memory session store with 2-hour expiry. **Revocation**: `checkRevocation()` queries `contractInst.GetState(index)` on every protected page load. **Activity reporting**: `reportPlatformActivity()` POSTs to Veriqid server so parent dashboard shows events. Routes: `/` (landing), `/signup` GET/POST (registration flow), `/login` GET/POST (auth flow), `/home` (protected video grid), `/profile` (pseudonymous account info), `/logout`, `/api/status` (JSON session check).

### demo-platform/templates/landing.html (~120 lines)
Kid-friendly landing page with hero section, "Sign up with Veriqid" CTA button, feature cards (zero data, parent control, unlinkable identity, instant verification), and 3-step "How It Works" section (now references 12-word mnemonic flow). Uses KidsTube red branding, not Veriqid teal/lime.

### demo-platform/templates/signup.html (~90 lines)
Shared signup/login template (controlled by `{{.IsLogin}}`). Contains hidden `veriqid-*` fields with IDs that match content script selectors, **including `veriqid-contract-index`** for revocation support. Visible fields: username input (signup only). Status indicator transitions from yellow "Waiting for extension..." to green "Verified!" when extension fills fields. Submit button starts disabled. Form POSTs to same URL (method-based routing).

### demo-platform/templates/home.html (~100 lines)
Protected video grid page. Shows welcome banner with username, verified badge, and 6 mock video cards (colorful gradient thumbnails with titles, channels, view counts, and durations). Hover effects with play overlay. Links to profile page.

### demo-platform/templates/profile.html (~80 lines)
Pseudonymous profile page. Avatar initial, username, Veriqid verification badge, account details (pseudonymous ID labeled "SPK", platform, session time). Privacy info panel with green checks (what KidsTube knows: "verified child") and gray crosses (what it doesn't: real name, exact age, parent info, other platform accounts, government ID). Note about ASC Unlinkability property.

### demo-platform/templates/revoked.html (~30 lines)
Displayed when parent revokes child's identity via Phase 6 dashboard. Lock icon, "Access Revoked" heading, explanation that parent/guardian revoked identity, suggestion to talk to parent. Link back to landing page.

### demo-platform/static/css/kidstube.css (~480 lines)
Complete KidsTube design system. Warm, kid-friendly palette (red primary, colorful accents). Covers all pages: landing (hero, features, how-it-works), auth (card-based signup/login), app (header, video grid, profile), and revoked state. Responsive breakpoints at 900px and 600px.

### demo-platform/static/js/kidstube.js (~60 lines)
Client-side logic for the auth page. Polls hidden Veriqid fields every 500ms to detect when the browser extension (Phase 5 or portable) has auto-filled proof values. Updates status indicator and enables submit button. Uses both polling and MutationObserver for reliability. Shows manual-mode hint after 10 seconds if extension not detected.

---

## How KidsTube Connects to Other Phases

### Phase 1 (Environment Setup)
KidsTube uses the exact same CGO crypto library (`libsecp256k1` with Boquila extensions) via `pkg/u2sso`. The `RegistrationVerify()` and `AuthVerify()` functions called in the KidsTube server are identical to those used in the Phase 1 proof-of-concept. These implement the ASC Verify() algorithm.

### Phase 2 (Bridge API)
The bridge must be running on `localhost:9090` for the browser extension to generate proofs (ASC Prove). KidsTube doesn't talk to the bridge directly — the extension does, then fills in the form fields that KidsTube reads. The bridge now supports `msk_hex` for the portable extension alongside the original `keypath` for Phase 5.

### Phase 3 (Enhanced Smart Contract)
KidsTube connects to the same Veriqid.sol contract as every other component. It calls `GetallActiveIDfromContract()` to build the anonymity set S for proof verification and `contractInst.GetState(index)` for **real-time revocation detection** on every protected page load. The contract binding uses `u2sso.NewVeriqid()` (the Veriqid.go binding, not the legacy U2SSO.go).

### Phase 4 (Veriqid Server)
KidsTube is architecturally identical to the Phase 4 server — both use `u2sso.CreateChallenge()`, `u2sso.RegistrationVerify()`, `u2sso.AuthVerify()`, SQLite, and cookie sessions. The key difference is the UI (kid-friendly vs. developer-focused) and the port (3000 vs. 8080). KidsTube also reports activity to the Phase 4 server via `POST /api/platform/activity`.

### Phase 5 (Browser Extension) & Portable Extension
Either extension works with KidsTube. Both detect the hidden `veriqid-*` form fields, call the bridge to generate proofs, and auto-fill them. The **portable extension** (`extension-portable/`) additionally fills the `veriqid-contract-index` field — this is how KidsTube learns which on-chain identity to check for revocation, without breaking pseudonymity (the platform knows the index but still can't derive the MPK from the SPK — ASC Anonymity preserved).

### Phase 6 (Parent Dashboard) — **Two-Way Connection**
This is the critical end-to-end link. The connection works in **both directions**:

**Direction 1 — KidsTube → Parent Dashboard (Activity Reporting):**
When a child registers on KidsTube, the server POSTs a notification to the Veriqid server at `POST /api/platform/activity`:
```json
{
  "service_name": "KidsTube",
  "spk_hex": "abc123...",
  "event_type": "registered",
  "contract_index": 3,
  "timestamp": "2026-03-12T10:30:00Z"
}
```
The Veriqid server stores this in the `platform_activity` table. When the parent loads their dashboard, `/api/parent/events` now includes these platform events alongside on-chain events, so the parent sees "Alex — Joined **KidsTube**" in their activity log.

**Direction 2 — Parent → KidsTube (Revocation Enforcement via Contract):**
When a parent clicks "Revoke" in the dashboard, it calls `revokeID(index)` on the smart contract, setting `active=false`. The next time the child loads any protected KidsTube page, `checkRevocation()` calls `contractInst.GetState(contractIndex)` and detects that the identity is no longer active. KidsTube immediately:
1. Clears the session cookie
2. Shows the "Access Revoked" page
3. Reports the revocation detection back to the Veriqid server

```
Parent Dashboard                     Smart Contract              KidsTube
     │                                    │                         │
     │ revokeID(index) ──────────────────►│ active = false          │
     │                                    │                         │
     │                                    │◄──── GetState(index) ───│
     │                                    │ returns false ─────────►│
     │                                    │                         │ → revoked.html
     │◄────── POST /api/platform/activity ┤                         │
     │  "revoked_detected on KidsTube"    │                         │
```

This architecture preserves privacy: KidsTube knows the contract_index but not the MPK. The Parent Dashboard knows the MPK and contract_index but not the platform-specific SPK. Neither can link the child's KidsTube identity to their identity on any other platform. This is the ASC Anonymity and Unlinkability properties working in concert.

### Portable Key System (Mnemonic Flow)
The portable key system (12-word mnemonic) connects to KidsTube through the portable extension:
1. Parent adds child → server generates mnemonic via `internal/mnemonic` → parent sees 12 words
2. Child pastes mnemonic into portable extension → extension derives MSK via SHA-256
3. Extension sends `msk_hex` to bridge → bridge generates proofs → extension fills KidsTube form
4. KidsTube verifies proof and creates account — same flow as Phase 5, different key source

---

## Next Steps: Phase 8 — JS SDK

With Phase 7 complete, you have a full end-to-end demo: parent dashboard + browser extension + consumer platform. Phase 8 creates the **JavaScript SDK** — a drop-in NPM package that any platform developer can integrate with `npm install veriqid-sdk`. This switches from the C/Go ring signature path to the JS/SNARK path (Semaphore + Circom), making integration possible without CGO or native dependencies.
