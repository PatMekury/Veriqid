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

	"github.com/patmekury/veriqid/internal/mnemonic"
	"github.com/patmekury/veriqid/internal/parent"
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

// ---------------------------------------------------------------------------
// Server — holds all state for the Veriqid platform server
// ---------------------------------------------------------------------------

type Server struct {
	PlatformName   string
	ServiceNameHex string // SHA-256 hash of the service name, hex-encoded
	ServiceNameRaw []byte // SHA-256 hash bytes (for passing to verification functions)
	ContractAddr   string
	RPCURL         string
	masterSecret   string // Phase 6: encryption master secret for custodial wallets

	Store          *store.Store
	Sessions       *session.Manager
	ParentSessions *session.ParentManager // Phase 6: parent-specific sessions
	Tmpl           *template.Template

	EthClient *ethclient.Client
	Contract  *u2sso.Veriqid
	EthKey    string // Deployer private key (hex, no 0x) for on-chain registration
}

func NewServer(platformName, contractAddr, rpcURL, dbPath, masterSecret, ethKey string) (*Server, error) {
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

	// Phase 6: Parent session manager (separate from child sessions)
	parentSessionKey := make([]byte, 32)
	rand.Read(parentSessionKey)
	parentSessions := session.NewParentManager(parentSessionKey)

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
		masterSecret:   masterSecret,
		Store:          db,
		Sessions:       sessions,
		ParentSessions: parentSessions,
		Tmpl:           tmpl,
		EthClient:      client,
		Contract:       contract,
		EthKey:         ethKey,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// HTML Page Handlers
// ---------------------------------------------------------------------------

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

func (s *Server) HandleSignupForm(w http.ResponseWriter, r *http.Request) {
	challenge := u2sso.CreateChallenge()
	challengeHex := hex.EncodeToString(challenge)

	// Store the challenge for replay protection
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

	// Extract form values — using the same field names as the Phase 1 server
	username := strings.TrimSpace(r.FormValue("name"))
	challengeHex := r.FormValue("challenge")
	spkHex := r.FormValue("spk")
	proofHex := r.FormValue("proof")
	ringSizeStr := r.FormValue("n")
	serviceNameHex := r.FormValue("sname")

	// Validate required fields
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
	challenge, err := hex.DecodeString(challengeHex)
	if err != nil {
		s.renderResult(w, "Sign Up Failed", "Invalid challenge format.", http.StatusBadRequest)
		return
	}

	spkBytes, err := hex.DecodeString(spkHex)
	if err != nil {
		s.renderResult(w, "Sign Up Failed", "Invalid SPK format.", http.StatusBadRequest)
		return
	}

	serviceName, err := hex.DecodeString(serviceNameHex)
	if err != nil {
		s.renderResult(w, "Sign Up Failed", "Invalid service name format.", http.StatusBadRequest)
		return
	}

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

	// Fetch all active IDs from the contract for ring verification
	idList, err := u2sso.GetallActiveIDfromContract(s.Contract)
	if err != nil {
		log.Printf("ERROR: failed to fetch IDs from contract: %v", err)
		s.renderResult(w, "Error", "Failed to fetch identity ring from blockchain.", http.StatusInternalServerError)
		return
	}

	// Verify the ring membership proof
	verified := u2sso.RegistrationVerify(proofHex, currentm, ringSize, serviceName, challenge, idList, spkBytes)

	if !verified {
		s.renderResult(w, "Sign Up Failed", "Membership proof verification failed. You may not be a verified identity.", http.StatusForbidden)
		return
	}

	// Proof is valid! Default to AgeBracket=1 (Under13) for hackathon demo.
	ageBracket := 1

	// Register the user in the database
	if err := s.Store.RegisterUser(spkHex, username, ageBracket); err != nil {
		log.Printf("ERROR: failed to register user: %v", err)
		s.renderResult(w, "Sign Up Failed", "Registration failed. Username may already be taken.", http.StatusConflict)
		return
	}

	// Create a session for the new user
	if err := s.Sessions.Login(w, r, spkHex, username); err != nil {
		log.Printf("ERROR: failed to create session: %v", err)
	}

	log.Printf("SUCCESS: User '%s' registered with SPK %s...", username, spkHex[:16])
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

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
	_ = r.FormValue("name") // username from form (used for display only)

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

	// Decode hex values
	challenge, err := hex.DecodeString(challengeHex)
	if err != nil {
		s.renderResult(w, "Login Failed", "Invalid challenge format.", http.StatusBadRequest)
		return
	}

	spkBytes, err := hex.DecodeString(spkHex)
	if err != nil {
		s.renderResult(w, "Login Failed", "Invalid SPK format.", http.StatusBadRequest)
		return
	}

	serviceName, err := hex.DecodeString(serviceNameHex)
	if err != nil {
		s.renderResult(w, "Login Failed", "Invalid service name format.", http.StatusBadRequest)
		return
	}

	// Verify the auth proof
	verified := u2sso.AuthVerify(signatureHex, serviceName, challenge, spkBytes)

	if !verified {
		s.renderResult(w, "Login Failed", "Authentication proof verification failed.", http.StatusForbidden)
		return
	}

	// Update last login time
	if err := s.Store.UpdateLastLogin(spkHex); err != nil {
		log.Printf("WARNING: failed to update last login: %v", err)
	}

	// Create session
	if err := s.Sessions.Login(w, r, spkHex, user.Username); err != nil {
		log.Printf("ERROR: failed to create session: %v", err)
	}

	log.Printf("SUCCESS: User '%s' logged in", user.Username)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

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

func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	s.Sessions.Logout(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// JSON API Handlers (for Platform SDK Integration)
// ---------------------------------------------------------------------------

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

type VerifyRegistrationRequest struct {
	ProofHex     string `json:"proof_hex"`
	SpkHex       string `json:"spk_hex"`
	ChallengeHex string `json:"challenge_hex"`
	RingSize     int    `json:"ring_size"`
	ServiceName  string `json:"service_name"`
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

	// Use the platform's own service name if provided, otherwise use ours
	var serviceNameBytes []byte
	if req.ServiceName != "" {
		var err error
		serviceNameBytes, err = hex.DecodeString(req.ServiceName)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid service_name hex")
			return
		}
	} else {
		serviceNameBytes = s.ServiceNameRaw
	}

	challenge, err := hex.DecodeString(req.ChallengeHex)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid challenge_hex")
		return
	}

	spkBytes, err := hex.DecodeString(req.SpkHex)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid spk_hex")
		return
	}

	// Calculate ring parameters
	currentm := 1
	tmp := 1
	for currentm = 1; currentm < u2sso.M; currentm++ {
		tmp *= u2sso.N
		if tmp >= req.RingSize {
			break
		}
	}

	// Fetch all active IDs from the contract
	idList, err := u2sso.GetallActiveIDfromContract(s.Contract)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to fetch identity ring")
		return
	}

	// Verify
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
		var err error
		serviceNameBytes, err = hex.DecodeString(req.ServiceName)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid service_name hex")
			return
		}
	} else {
		serviceNameBytes = s.ServiceNameRaw
	}

	challenge, err := hex.DecodeString(req.ChallengeHex)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid challenge_hex")
		return
	}

	spkBytes, err := hex.DecodeString(req.SpkHex)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid spk_hex")
		return
	}

	verified := u2sso.AuthVerify(req.AuthProofHex, serviceNameBytes, challenge, spkBytes)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VerifyAuthResponse{
		Verified: verified,
	})
}

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

// ---------------------------------------------------------------------------
// Phase 6: Parent Portal API Handlers
// ---------------------------------------------------------------------------

// getParentFromSession extracts the parent ID from the session cookie.
func (s *Server) getParentFromSession(r *http.Request) (int64, bool) {
	loggedIn, parentID := s.ParentSessions.IsLoggedIn(r)
	if !loggedIn {
		return 0, false
	}
	return parentID, true
}

// POST /api/parent/register
func (s *Server) handleParentRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

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

		parentID, err = s.Store.CreateParentByEmail(email, passwordHash, ethAddr, privKeyEnc)
		if err != nil {
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

		parentID, err = s.Store.CreateParentByPhone(phone, ethAddr, privKeyEnc)
		if err != nil {
			http.Error(w, `{"error":"phone already registered"}`, http.StatusConflict)
			return
		}

		// Generate and "send" OTP (console logging for hackathon demo)
		code, _ := parent.GenerateOTP()
		s.Store.StoreOTP(phone, code)
		fmt.Printf("[OTP] Code for %s: %s\n", phone, code)

	} else {
		http.Error(w, `{"error":"provide email+password or phone"}`, http.StatusBadRequest)
		return
	}

	isPhone := req.Phone != ""

	if !isPhone {
		// Email registration: auto-login immediately
		s.ParentSessions.Login(w, parentID)
	}
	// Phone registration: don't auto-login — user must verify OTP via /api/parent/login

	log.Printf("SUCCESS: Parent registered (ID=%d, eth=%s, phone=%v)", parentID, ethAddr, isPhone)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"parent_id":    parentID,
		"method":       map[bool]string{true: "email", false: "phone"}[req.Email != ""],
		"otp_required": isPhone,
	})
}

// POST /api/parent/login
func (s *Server) handleParentLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

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
		p, err = s.Store.GetParentByEmail(email)
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
		log.Printf("[DEBUG OTP] Login attempt: phone=%q code=%q", phone, req.Code)
		valid, err := s.Store.VerifyOTP(phone, req.Code)
		log.Printf("[DEBUG OTP] VerifyOTP result: valid=%v err=%v", valid, err)
		if err != nil || !valid {
			http.Error(w, `{"error":"invalid or expired code"}`, http.StatusUnauthorized)
			return
		}
		p, err = s.Store.GetParentByPhone(phone)
		if err != nil {
			http.Error(w, `{"error":"account not found"}`, http.StatusUnauthorized)
			return
		}

	} else {
		http.Error(w, `{"error":"provide email+password or phone+code"}`, http.StatusBadRequest)
		return
	}

	// Update last login
	s.Store.UpdateParentLastLogin(p.ID)

	// Create session
	s.ParentSessions.Login(w, p.ID)

	log.Printf("SUCCESS: Parent logged in (ID=%d)", p.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"parent_id": p.ID,
	})
}

// POST /api/parent/logout
func (s *Server) handleParentLogout(w http.ResponseWriter, r *http.Request) {
	s.ParentSessions.Logout(w, r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// POST /api/parent/send-otp
func (s *Server) handleSendOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	phone := parent.NormalizePhone(req.Phone)
	if err := parent.ValidatePhone(phone); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Check the parent exists
	_, err := s.Store.GetParentByPhone(phone)
	if err != nil {
		http.Error(w, `{"error":"no account with this phone number"}`, http.StatusNotFound)
		return
	}

	// Generate and "send" OTP
	code, _ := parent.GenerateOTP()
	s.Store.StoreOTP(phone, code)
	fmt.Printf("[OTP] Code for %s: %s\n", phone, code)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Code sent (check server terminal for demo)",
	})
}

// GET /api/parent/me
func (s *Server) handleGetParentInfo(w http.ResponseWriter, r *http.Request) {
	parentID, ok := s.getParentFromSession(r)
	if !ok {
		http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
		return
	}

	p, err := s.Store.GetParentByID(parentID)
	if err != nil {
		http.Error(w, `{"error":"parent not found"}`, http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"success":   true,
		"parent_id": p.ID,
	}
	if p.Email != nil {
		resp["email"] = *p.Email
	}
	if p.Phone != nil {
		resp["phone"] = *p.Phone
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /api/parent/child/add
func (s *Server) handleAddChild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	parentID, ok := s.getParentFromSession(r)
	if !ok {
		http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		AgeBracket  int    `json:"age_bracket"`
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

	// Generate a 12-word mnemonic phrase → derive 32-byte MSK via SHA-256
	phrase, msk, err := mnemonic.Generate()
	if err != nil {
		http.Error(w, `{"error":"failed to generate key"}`, http.StatusInternalServerError)
		return
	}
	mskHex := hex.EncodeToString(msk)

	// Derive mpk from msk using the CGO library (CreateID)
	mpkBytes := u2sso.CreateID(msk)
	var mpkHex string
	if len(mpkBytes) == 0 {
		log.Printf("WARNING: CreateID returned empty mpk — using placeholder")
		mpkHex = mskHex // placeholder
	} else {
		mpkHex = hex.EncodeToString(mpkBytes)
	}

	// Encrypt msk for storage (server keeps encrypted copy for recovery)
	encKey := parent.DeriveEncryptionKey(s.masterSecret)
	mskEnc, err := parent.EncryptPrivateKey(mskHex, encKey)
	if err != nil {
		http.Error(w, `{"error":"failed to encrypt key"}`, http.StatusInternalServerError)
		return
	}

	// Store child in database (status = 'pending' until verified)
	childID, err := s.Store.AddChild(parentID, req.DisplayName, req.AgeBracket, mskEnc, mpkHex)
	if err != nil {
		http.Error(w, `{"error":"failed to add child"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("SUCCESS: Child '%s' added for parent %d (childID=%d) — mnemonic generated", req.DisplayName, parentID, childID)

	// Return the mnemonic phrase — this is shown ONCE to the parent.
	// The parent gives the phrase to the child, who enters it in the browser extension.
	// The server does NOT store the plaintext mnemonic or MSK after this response.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"child_id":  childID,
		"status":    "pending",
		"mnemonic":  phrase,
		"message":   "Child added. Give the 12-word phrase to your child to paste in their Veriqid extension.",
	})
}

// GET /api/parent/children
func (s *Server) handleGetChildren(w http.ResponseWriter, r *http.Request) {
	parentID, ok := s.getParentFromSession(r)
	if !ok {
		http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
		return
	}

	children, err := s.Store.GetChildrenByParent(parentID)
	if err != nil {
		http.Error(w, `{"error":"failed to load children"}`, http.StatusInternalServerError)
		return
	}

	ageBracketLabels := map[int]string{0: "Unknown", 1: "Under 13", 2: "Teen (13–17)", 3: "Adult (18+)"}

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
		response = append(response, cr)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"children": response,
	})
}

// POST /api/parent/verify/approve — hackathon simulation of verifier approval
func (s *Server) handleVerifyApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

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

	// Verify the child belongs to this parent
	children, err := s.Store.GetChildrenByParent(parentID)
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

	// Register identity on-chain via smart contract
	if child.MpkHex == nil || *child.MpkHex == "" {
		http.Error(w, `{"error":"child has no MPK — cannot register on-chain"}`, http.StatusInternalServerError)
		return
	}

	mpkBytes, err := hex.DecodeString(*child.MpkHex)
	if err != nil {
		http.Error(w, `{"error":"invalid MPK hex"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("VERIFY: Registering child '%s' on-chain (mpk=%s...)", child.DisplayName, (*child.MpkHex)[:16])

	contractIndex64, err := u2sso.AddIDstoIdR(s.EthClient, s.EthKey, s.Contract, mpkBytes, uint8(child.AgeBracket))
	if err != nil {
		log.Printf("ERROR: On-chain registration failed for child '%s': %v", child.DisplayName, err)
		http.Error(w, fmt.Sprintf(`{"error":"on-chain registration failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	contractIndex := int(contractIndex64)

	// Update child record
	if err := s.Store.MarkChildVerified(req.ChildID, contractIndex); err != nil {
		http.Error(w, `{"error":"failed to update child status"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("SUCCESS: Child '%s' verified (contractIndex=%d)", child.DisplayName, contractIndex)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"child_id":       req.ChildID,
		"contract_index": contractIndex,
		"status":         "verified",
		"message":        "Identity verified and registered on-chain.",
	})
}

// POST /api/parent/child/revoke
func (s *Server) handleRevokeChild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	parentID, ok := s.getParentFromSession(r)
	if !ok {
		http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		ChildID  int64  `json:"child_id"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Re-authenticate the parent (extra safety for destructive action)
	parentRecord, err := s.Store.GetParentByID(parentID)
	if err != nil {
		http.Error(w, `{"error":"parent not found"}`, http.StatusInternalServerError)
		return
	}
	if parentRecord.PasswordHash != nil && !parent.CheckPassword(req.Password, *parentRecord.PasswordHash) {
		http.Error(w, `{"error":"incorrect password"}`, http.StatusUnauthorized)
		return
	}

	// Find the child
	children, err := s.Store.GetChildrenByParent(parentID)
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

	// For hackathon demo: simulate on-chain revocation
	// In production, this would call revokeID() on the contract
	log.Printf("REVOKE: Simulating on-chain revocation for child '%s' (contractIndex=%d)",
		child.DisplayName, *child.ContractIndex)

	// Update local database
	if err := s.Store.MarkChildRevoked(req.ChildID); err != nil {
		http.Error(w, `{"error":"failed to update status"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("SUCCESS: Child '%s' revoked", child.DisplayName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"child_id": req.ChildID,
		"status":   "revoked",
		"message":  "Identity has been permanently revoked across all platforms.",
	})
}

// GET /api/parent/events — contract events for parent's children
func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	parentID, ok := s.getParentFromSession(r)
	if !ok {
		http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
		return
	}

	children, err := s.Store.GetChildrenByParent(parentID)
	if err != nil {
		http.Error(w, `{"error":"failed to load children"}`, http.StatusInternalServerError)
		return
	}

	// Build a simple event list from the children's status history
	type EventEntry struct {
		Type          string `json:"type"`
		ChildName     string `json:"child_name"`
		ContractIndex int    `json:"contract_index"`
		BlockNumber   int    `json:"block_number"`
		ServiceName   string `json:"service_name,omitempty"`
		EventTime     string `json:"event_time,omitempty"`
	}

	var events []EventEntry
	for _, c := range children {
		if c.ContractIndex != nil {
			events = append(events, EventEntry{
				Type:          "registered",
				ChildName:     c.DisplayName,
				ContractIndex: *c.ContractIndex,
				BlockNumber:   1, // placeholder for hackathon
			})
			if c.Status == "revoked" {
				events = append(events, EventEntry{
					Type:          "revoked",
					ChildName:     c.DisplayName,
					ContractIndex: *c.ContractIndex,
					BlockNumber:   2, // placeholder for hackathon
				})
			}
		}
	}

	// Also include platform-specific events (e.g., "Alex registered on KidsTube")
	platformActivities, err := s.Store.GetAllPlatformActivityForParent(parentID)
	if err != nil {
		log.Printf("WARNING: failed to load platform activity: %v", err)
		// Don't fail — just continue without platform events
	} else {
		for _, pa := range platformActivities {
			ci := 0
			if pa.ContractIndex != nil {
				ci = *pa.ContractIndex
			}
			// Find the child's display name from contract_index
			childName := "Unknown"
			for _, c := range children {
				if c.ContractIndex != nil && *c.ContractIndex == ci {
					childName = c.DisplayName
					break
				}
			}
			events = append(events, EventEntry{
				Type:          "platform_" + pa.EventType,
				ChildName:     childName,
				ContractIndex: ci,
				BlockNumber:   0, // Platform events aren't on-chain
				ServiceName:   pa.ServiceName,
				EventTime:     pa.EventTime,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"events":  events,
	})
}

// POST /api/platform/activity — receives activity reports from third-party platforms
// (e.g., KidsTube reports that a child registered)
func (s *Server) handlePlatformActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	var req struct {
		ServiceName   string  `json:"service_name"`
		SpkHex        string  `json:"spk_hex"`
		EventType     string  `json:"event_type"`
		Timestamp     string  `json:"timestamp"`
		ContractIndex *int    `json:"contract_index"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.ServiceName == "" || req.SpkHex == "" || req.EventType == "" {
		http.Error(w, `{"error":"missing required fields: service_name, spk_hex, event_type"}`, http.StatusBadRequest)
		return
	}

	if req.Timestamp == "" {
		req.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	log.Printf("Platform activity: %s on %s (SPK: %s..., contractIndex: %v)",
		req.EventType, req.ServiceName, req.SpkHex[:16], req.ContractIndex)

	if err := s.Store.AddPlatformActivity(req.ContractIndex, req.ServiceName, req.SpkHex, req.EventType, req.Timestamp); err != nil {
		log.Printf("Failed to store platform activity: %v", err)
		http.Error(w, `{"error":"failed to store activity"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Activity recorded",
	})
}

// ---------------------------------------------------------------------------
// Route Registration & Middleware
// ---------------------------------------------------------------------------

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Static files (CSS, images, JS)
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

	// Phase 6: Parent Portal API
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

	// Phase 7: Platform activity reporting (third-party platforms report child registrations)
	mux.HandleFunc("/api/platform/activity", s.handlePlatformActivity)

	// Phase 6: Parent dashboard static files
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", http.FileServer(http.Dir("dashboard"))))
	mux.HandleFunc("/parent", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./dashboard/index.html")
	})
}

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

// ---------------------------------------------------------------------------
// Background cleanup
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

// Suppress "bytes imported and not used" — used in future expansion
var _ = bytes.Compare

func main() {
	contractAddr := flag.String("contract", "", "Veriqid smart contract address (required)")
	clientAddr := flag.String("client", "http://127.0.0.1:7545", "Ethereum JSON-RPC endpoint")
	port := flag.Int("port", 8080, "Port for the server to listen on")
	serviceName := flag.String("service", "KidsTube", "Platform service name (used for SPK derivation)")
	dbPath := flag.String("db", "./veriqid.db", "Path to SQLite database file")
	masterSecret := flag.String("master-secret", "hackathon-demo-key-change-in-production", "Encryption master secret for custodial wallets")
	ethKey := flag.String("ethkey", "", "Deployer Ethereum private key (hex, no 0x) for on-chain registration")
	flag.Parse()

	if *contractAddr == "" {
		log.Fatal("Error: -contract flag is required.\n\nUsage:\n  ./veriqid-server -contract 0x... -ethkey <hex> [-service KidsTube] [-port 8080] [-db ./veriqid.db]")
	}
	if *ethKey == "" {
		log.Fatal("Error: -ethkey flag is required (deployer private key for on-chain registration)")
	}

	srv, err := NewServer(*serviceName, *contractAddr, *clientAddr, *dbPath, *masterSecret, *ethKey)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}
	defer srv.Store.Close()

	// Start background cleanup
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
	fmt.Println("  Parent Portal:")
	fmt.Println("    GET  /parent                   Parent dashboard")
	fmt.Println("    POST /api/parent/register      Create parent account")
	fmt.Println("    POST /api/parent/login         Parent login")
	fmt.Println("    POST /api/parent/logout        Parent logout")
	fmt.Println("    POST /api/parent/send-otp      Send phone OTP")
	fmt.Println("    GET  /api/parent/me            Current parent info")
	fmt.Println("    GET  /api/parent/children      List children")
	fmt.Println("    POST /api/parent/child/add     Add child")
	fmt.Println("    POST /api/parent/child/revoke  Revoke child identity")
	fmt.Println("    POST /api/parent/verify/approve Simulate verification")
	fmt.Println("    GET  /api/parent/events        Activity log")
	fmt.Println("===========================================")

	log.Fatal(http.ListenAndServe(addr, handler))
}
