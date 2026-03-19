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

	// Verify connection
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

// Signup page — generates challenge, serves form with hidden Veriqid fields
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

// Signup form submission — verify proof, create account
func handleSignupSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	proofHex := r.FormValue("veriqid-proof")
	spkHex := r.FormValue("veriqid-spk")
	challengeHex := r.FormValue("veriqid-challenge")
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

	// Check if SPK is already registered (Sybil guard — one account per identity per platform)
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

	// Parse ring size (total number of identities on-chain)
	var currentN int
	fmt.Sscanf(nStr, "%d", &currentN)
	if currentN < 2 {
		currentN = 2 // Minimum ring size
	}

	// Calculate m: smallest m where N^m >= currentN
	// This MUST match the bridge/CLI calculation (N=2 constant from u2sso)
	currentM := 1
	ringSize := 1
	for i := 1; i < 10; i++ { // 10 = u2sso.M max
		ringSize = 2 * ringSize // 2 = u2sso.N
		if ringSize >= currentN {
			currentM = i
			break
		}
	}

	// Fetch all active IDs from the contract for ring verification
	idList, err := u2sso.GetallActiveIDfromContract(contractInst)
	if err != nil {
		log.Printf("Failed to fetch IDs from contract: %v", err)
		http.Error(w, "Failed to verify identity — contract unavailable", http.StatusInternalServerError)
		return
	}

	// ─── VERIFY THE RING MEMBERSHIP PROOF ───
	log.Printf("DEBUG VERIFY: currentM=%d, currentN=%d, nStr=%q, idListLen=%d, proofLen=%d, spkLen=%d, snameLen=%d, chalLen=%d",
		currentM, currentN, nStr, len(idList), len(proofHex), len(spkHex), len(snameHex), len(challengeHex))
	valid := u2sso.RegistrationVerify(proofHex, currentM, currentN, snameBytes, challengeBytes, idList, spkBytes)

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

// Login form submission — verify auth proof
func handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// The content script fills veriqid-auth-proof; legacy forms may use veriqid-signature
	signatureHex := r.FormValue("veriqid-auth-proof")
	if signatureHex == "" {
		signatureHex = r.FormValue("veriqid-signature")
	}
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

	// Check that this SPK is registered on KidsTube
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

// checkRevocation queries the smart contract to see if the identity's mpk
// has been revoked. Uses the contract_index stored at signup time.
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
	// The privacy architecture intentionally prevents this linkage.
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
	http.HandleFunc("/signup", handleSignup)
	http.HandleFunc("/login", handleLogin)

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
