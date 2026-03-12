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

// ---------------------------------------------------------------------------
// Server — holds all state for the Veriqid platform server
// ---------------------------------------------------------------------------

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
	flag.Parse()

	if *contractAddr == "" {
		log.Fatal("Error: -contract flag is required.\n\nUsage:\n  ./veriqid-server -contract 0x... [-service KidsTube] [-port 8080] [-db ./veriqid.db]")
	}

	srv, err := NewServer(*serviceName, *contractAddr, *clientAddr, *dbPath)
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
	fmt.Println("===========================================")

	log.Fatal(http.ListenAndServe(addr, handler))
}
