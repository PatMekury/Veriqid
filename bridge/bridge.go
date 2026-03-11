// Package bridge provides the Veriqid Bridge API (Phase 2).
// This HTTP API wraps the CGO crypto library for browser/extension communication.
//
// The bridge runs LOCALLY on the user's machine (like a password manager).
// The master secret key never leaves the device.
//
// Endpoints:
//
//	GET  /api/status              - Health check and configuration info
//	POST /api/identity/create     - Generate msk, register mpk on-chain
//	POST /api/identity/register   - Generate membership proof + spk for signup
//	POST /api/identity/auth       - Generate auth proof for login
//	POST /api/identity/list       - List available identity key files
//	GET  /api/identity/challenge  - Generate random 32-byte challenge
package bridge

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"

	u2sso "github.com/patmekury/veriqid/pkg/u2sso"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
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

// ============================================================
// Bridge Core
// ============================================================

// Bridge is the main API handler holding default configuration.
type Bridge struct {
	// DefaultContract is the default smart contract address (with 0x prefix).
	DefaultContract string

	// DefaultRPCURL is the default Ethereum JSON-RPC endpoint.
	DefaultRPCURL string
}

// NewBridge creates a new Bridge instance with the given defaults.
func NewBridge(contract, rpcURL string) *Bridge {
	if rpcURL == "" {
		rpcURL = "http://127.0.0.1:7545"
	}
	return &Bridge{
		DefaultContract: contract,
		DefaultRPCURL:   rpcURL,
	}
}

// resolveRPCURL returns the request's RPC URL or falls back to the default.
func (b *Bridge) resolveRPCURL(reqURL string) string {
	if reqURL != "" {
		return reqURL
	}
	return b.DefaultRPCURL
}

// resolveContract returns the request's contract or falls back to the default.
func (b *Bridge) resolveContract(reqContract string) string {
	if reqContract != "" {
		return reqContract
	}
	return b.DefaultContract
}

// connectToContract dials the Ethereum client and creates the contract instance.
// This mirrors the setup logic in cmd/client/main.go lines 59-90.
func connectToContract(rpcURL, contractAddr string) (*ethclient.Client, *u2sso.Veriqid, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Ethereum client at %s: %w", rpcURL, err)
	}

	address := common.HexToAddress(contractAddr)
	bytecode, err := client.CodeAt(context.Background(), address, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check contract at %s: %w", contractAddr, err)
	}
	if len(bytecode) == 0 {
		return nil, nil, fmt.Errorf("no contract found at address %s", contractAddr)
	}

	instance, err := u2sso.NewVeriqid(address, client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to instantiate Veriqid contract: %w", err)
	}

	return client, instance, nil
}

// ============================================================
// Request Types
// ============================================================

// CreateIdentityRequest holds the parameters for creating a new identity.
type CreateIdentityRequest struct {
	// Keypath is the filesystem path where the 32-byte msk will be saved.
	Keypath string `json:"keypath"`

	// EthKey is the Ethereum private key (hex, WITHOUT 0x prefix) for gas.
	EthKey string `json:"ethkey"`

	// Contract is the Veriqid contract address (WITH 0x prefix). Optional if default set.
	Contract string `json:"contract,omitempty"`

	// RPCURL is the Ethereum JSON-RPC endpoint. Optional if default set.
	RPCURL string `json:"rpc_url,omitempty"`

	// AgeBracket is the age category: 0=Unknown, 1=Under13, 2=Teen, 3=Adult.
	AgeBracket uint8 `json:"age_bracket"`
}

// RegisterRequest holds the parameters for generating a registration proof.
type RegisterRequest struct {
	// Keypath to the master secret key file (must already exist).
	Keypath string `json:"keypath"`

	// ServiceName is the hex-encoded SHA-256 hash of the service name.
	ServiceName string `json:"service_name"`

	// Challenge is the hex-encoded 32-byte random challenge from the server.
	Challenge string `json:"challenge"`

	// Contract address. Optional if default set.
	Contract string `json:"contract,omitempty"`

	// RPCURL. Optional if default set.
	RPCURL string `json:"rpc_url,omitempty"`
}

// AuthRequest holds the parameters for generating an authentication proof.
type AuthRequest struct {
	// Keypath to the master secret key file.
	Keypath string `json:"keypath"`

	// ServiceName is the hex-encoded SHA-256 hash of the service name.
	ServiceName string `json:"service_name"`

	// Challenge is the hex-encoded 32-byte random challenge.
	Challenge string `json:"challenge"`
}

// ListKeysRequest holds the parameters for listing available identity keys.
type ListKeysRequest struct {
	// KeyDir is the directory to scan for key files.
	KeyDir string `json:"key_dir"`

	// Contract address for on-chain status checks. Optional.
	Contract string `json:"contract,omitempty"`

	// RPCURL. Optional.
	RPCURL string `json:"rpc_url,omitempty"`
}

// ============================================================
// Response Types
// ============================================================

// CreateIdentityResponse is returned after creating and registering an identity.
type CreateIdentityResponse struct {
	Success bool   `json:"success"`
	MpkHex  string `json:"mpk_hex,omitempty"`
	Index   int64  `json:"index,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RegisterResponse is returned after generating a registration proof.
type RegisterResponse struct {
	Success  bool   `json:"success"`
	ProofHex string `json:"proof_hex,omitempty"`
	SpkHex   string `json:"spk_hex,omitempty"`
	RingSize int64  `json:"ring_size,omitempty"`
	Error    string `json:"error,omitempty"`
}

// AuthResponse is returned after generating an authentication proof.
type AuthResponse struct {
	Success      bool   `json:"success"`
	AuthProofHex string `json:"auth_proof_hex,omitempty"`
	SpkHex       string `json:"spk_hex,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ChallengeResponse is returned when requesting a fresh random challenge.
type ChallengeResponse struct {
	Challenge string `json:"challenge"`
	Error     string `json:"error,omitempty"`
}

// StatusResponse is returned by the health check endpoint.
type StatusResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Contract string `json:"contract,omitempty"`
	RPCURL   string `json:"rpc_url,omitempty"`
}

// KeyInfo represents a single identity key file with its on-chain status.
type KeyInfo struct {
	Keypath string `json:"keypath"`
	MpkHex  string `json:"mpk_hex"`
	Active  bool   `json:"active"`
	Index   int64  `json:"index"`
}

// ListKeysResponse is returned with available identity keys.
type ListKeysResponse struct {
	Success bool      `json:"success"`
	Keys    []KeyInfo `json:"keys,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// ============================================================
// Helpers
// ============================================================

// writeError sends a JSON error response with the given HTTP status code.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// writeJSON sends a JSON success response.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// ============================================================
// CORS Middleware
// ============================================================

// CORSMiddleware adds CORS headers so the browser extension can
// communicate with the local bridge from its chrome-extension:// origin.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight (OPTIONS) requests immediately
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ============================================================
// Handlers
// ============================================================

// HandleStatus returns bridge health and configuration info.
// GET /api/status
func (b *Bridge) HandleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, StatusResponse{
		Status:   "ok",
		Version:  "0.2.0",
		Contract: b.DefaultContract,
		RPCURL:   b.DefaultRPCURL,
	})
}

// HandleCreateIdentity creates a new msk, derives mpk, and registers on-chain.
// POST /api/identity/create
//
// This mirrors cmd/client/main.go lines 92-113 ("create" command):
//  1. CreatePasskey(keypath)  → 32 random bytes via RAND_bytes, saved to file
//  2. LoadPasskey(keypath)    → read back the 32-byte msk
//  3. CreateID(mskBytes)      → derive 33-byte compressed mpk
//  4. AddIDstoIdR(...)        → Ethereum transaction to register mpk on-chain
func (b *Bridge) HandleCreateIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed, use POST")
		return
	}

	var req CreateIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Keypath == "" {
		writeError(w, http.StatusBadRequest, "keypath is required")
		return
	}
	if req.EthKey == "" {
		writeError(w, http.StatusBadRequest, "ethkey is required")
		return
	}

	contract := b.resolveContract(req.Contract)
	rpcURL := b.resolveRPCURL(req.RPCURL)
	if contract == "" {
		writeError(w, http.StatusBadRequest, "contract address is required (pass in request or set as default)")
		return
	}

	// Connect to Ethereum and the contract
	ethClient, instance, err := connectToContract(rpcURL, contract)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = ethClient // used implicitly by instance

	// Step 1: Create 32-byte msk via OpenSSL RAND_bytes, save to disk
	if err := u2sso.CreatePasskey(req.Keypath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create passkey: "+err.Error())
		return
	}

	// Step 2: Load msk back from disk
	mskBytes, ok := u2sso.LoadPasskey(req.Keypath)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to load passkey after creation")
		return
	}

	// Step 3: Derive 33-byte compressed mpk via secp256k1_boquila_gen_mpk
	mpkBytes := u2sso.CreateID(mskBytes)

	// Step 4: Register mpk on the smart contract (Ethereum tx, costs gas)
	index, err := u2sso.AddIDstoIdR(ethClient, req.EthKey, instance, mpkBytes, req.AgeBracket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register on-chain: "+err.Error())
		return
	}

	writeJSON(w, CreateIdentityResponse{
		Success: true,
		MpkHex:  hex.EncodeToString(mpkBytes),
		Index:   index,
	})
}

// HandleRegister generates a registration proof (ring membership + spk).
// POST /api/identity/register
//
// This mirrors cmd/client/main.go lines 128-193 ("register" command):
//  1. LoadPasskey(keypath)        → load msk
//  2. CreateID(mskBytes)          → derive mpk (to find our index)
//  3. GetIDIndexfromContract(...)  → find our position in the ring
//  4. Calculate ring parameters (currentm from idsize)
//  5. GetallActiveIDfromContract() → fetch all active IDs (the ring)
//  6. RegistrationProof(...)       → generate proof + spk
func (b *Bridge) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed, use POST")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Keypath == "" || req.ServiceName == "" || req.Challenge == "" {
		writeError(w, http.StatusBadRequest, "keypath, service_name, and challenge are all required")
		return
	}

	contract := b.resolveContract(req.Contract)
	rpcURL := b.resolveRPCURL(req.RPCURL)
	if contract == "" {
		writeError(w, http.StatusBadRequest, "contract address is required")
		return
	}

	// Decode hex inputs to bytes
	serviceName, err := hex.DecodeString(req.ServiceName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "service_name must be valid hex: "+err.Error())
		return
	}
	challenge, err := hex.DecodeString(req.Challenge)
	if err != nil {
		writeError(w, http.StatusBadRequest, "challenge must be valid hex: "+err.Error())
		return
	}

	// Connect to contract
	_, instance, err := connectToContract(rpcURL, contract)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Load the master secret key
	mskBytes, ok := u2sso.LoadPasskey(req.Keypath)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to load passkey from "+req.Keypath)
		return
	}

	// Get total ID count from contract
	idsize, err := instance.GetIDSize(nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get ID size from contract: "+err.Error())
		return
	}
	if idsize.Int64() < 2 {
		writeError(w, http.StatusBadRequest, "at least 2 registered identities are required for ring signatures")
		return
	}

	// Calculate ring parameters: currentm = smallest m where N^m >= idsize
	// This mirrors cmd/client/main.go lines 164-172
	currentm := 1
	ringSize := 1
	for i := 1; i < u2sso.M; i++ {
		ringSize = u2sso.N * ringSize
		if ringSize >= int(idsize.Int64()) {
			currentm = i
			break
		}
	}

	// Derive mpk to find our index in the ring
	mpkBytes := u2sso.CreateID(mskBytes)
	index, err := u2sso.GetIDIndexfromContract(instance, mpkBytes)
	if err != nil || index == -1 {
		writeError(w, http.StatusBadRequest, "this passkey's identity is not registered on the contract")
		return
	}

	// Fetch all active identities (the ring)
	idList, err := u2sso.GetallActiveIDfromContract(instance)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch IDs from contract: "+err.Error())
		return
	}

	// Generate the registration proof bundle
	// Returns: proofHex (string), spkBytes ([]byte), success (bool)
	proofHex, spkBytes, ok := u2sso.RegistrationProof(
		int(index),
		currentm,
		int(idsize.Int64()),
		serviceName,
		challenge,
		mskBytes,
		idList,
	)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to generate registration proof")
		return
	}

	writeJSON(w, RegisterResponse{
		Success:  true,
		ProofHex: proofHex,
		SpkHex:   hex.EncodeToString(spkBytes),
		RingSize: idsize.Int64(),
	})
}

// HandleAuth generates an authentication proof (65-byte Boquila proof).
// POST /api/identity/auth
//
// This mirrors cmd/client/main.go lines 195-228 ("auth" command):
//  1. LoadPasskey(keypath)    → load msk
//  2. AuthProof(sname, chal, msk) → 65-byte Boquila auth proof + spk (returned as hex)
//
// Auth is simpler than register — no ring needed. It proves
// "I control the secret key behind this spk" for a given challenge.
func (b *Bridge) HandleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed, use POST")
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Keypath == "" || req.ServiceName == "" || req.Challenge == "" {
		writeError(w, http.StatusBadRequest, "keypath, service_name, and challenge are all required")
		return
	}

	// Decode hex inputs
	serviceName, err := hex.DecodeString(req.ServiceName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "service_name must be valid hex: "+err.Error())
		return
	}
	challenge, err := hex.DecodeString(req.Challenge)
	if err != nil {
		writeError(w, http.StatusBadRequest, "challenge must be valid hex: "+err.Error())
		return
	}

	// Load master secret key
	mskBytes, ok := u2sso.LoadPasskey(req.Keypath)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to load passkey from "+req.Keypath)
		return
	}

	// Generate auth proof
	// AuthProof signature: func AuthProof(serviceName []byte, challenge []byte, mskBytes []byte) (string, bool)
	// Returns the 65-byte proof already hex-encoded, and a success bool.
	// NOTE: AuthProof does NOT return the spk separately — it's embedded in the proof flow.
	// The spk can be re-derived deterministically from the same msk + service_name.
	authProofHex, ok := u2sso.AuthProof(serviceName, challenge, mskBytes)
	if !ok {
		writeError(w, http.StatusInternalServerError, "failed to generate auth proof")
		return
	}

	writeJSON(w, AuthResponse{
		Success:      true,
		AuthProofHex: authProofHex,
	})
}

// HandleChallenge generates a random 32-byte challenge.
// GET /api/identity/challenge
//
// NOTE: In practice, the service provider's server generates challenges.
// This endpoint exists for testing and local verification scenarios.
func (b *Bridge) HandleChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed, use GET")
		return
	}

	// 32 random bytes via OpenSSL RAND_bytes (through CGO)
	challengeBytes := u2sso.CreateChallenge()

	writeJSON(w, ChallengeResponse{
		Challenge: hex.EncodeToString(challengeBytes),
	})
}

// HandleListKeys scans a directory for 32-byte key files and checks on-chain status.
// POST /api/identity/list
func (b *Bridge) HandleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed, use POST")
		return
	}

	var req ListKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.KeyDir == "" {
		writeError(w, http.StatusBadRequest, "key_dir is required")
		return
	}

	contract := b.resolveContract(req.Contract)
	rpcURL := b.resolveRPCURL(req.RPCURL)

	// Optionally connect to contract for on-chain status checks
	var instance *u2sso.Veriqid
	if contract != "" {
		_, inst, err := connectToContract(rpcURL, contract)
		if err == nil {
			instance = inst
		}
	}

	// Scan the directory for key files
	entries, err := os.ReadDir(req.KeyDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read key directory: "+err.Error())
		return
	}

	var keys []KeyInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only consider files that are exactly 32 bytes (valid msk files)
		info, err := entry.Info()
		if err != nil || info.Size() != 32 {
			continue
		}

		keypath := filepath.Join(req.KeyDir, entry.Name())

		// Load the msk and derive mpk
		mskBytes, ok := u2sso.LoadPasskey(keypath)
		if !ok {
			continue
		}

		mpkBytes := u2sso.CreateID(mskBytes)

		ki := KeyInfo{
			Keypath: keypath,
			MpkHex:  hex.EncodeToString(mpkBytes),
			Index:   -1,
			Active:  false,
		}

		// If contract instance is available, check on-chain status
		if instance != nil {
			// Split mpk into uint256 + uint for contract lookup
			byte32 := new(big.Int).SetBytes(mpkBytes[:32])
			byte33 := new(big.Int).SetBytes(mpkBytes[32:])
			idx, err := instance.GetIDIndex(nil, byte32, byte33)
			if err == nil && idx.Int64() >= 0 {
				ki.Index = idx.Int64()
				// Check if active
				state, stateErr := instance.GetState(nil, idx)
				if stateErr == nil {
					ki.Active = state
				}
			}
		}

		keys = append(keys, ki)
	}

	writeJSON(w, ListKeysResponse{
		Success: true,
		Keys:    keys,
	})
}

// ============================================================
// Route Registration
// ============================================================

// RegisterRoutes sets up all the bridge API routes on the given ServeMux.
func (b *Bridge) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/status", b.HandleStatus)
	mux.HandleFunc("/api/identity/create", b.HandleCreateIdentity)
	mux.HandleFunc("/api/identity/register", b.HandleRegister)
	mux.HandleFunc("/api/identity/auth", b.HandleAuth)
	mux.HandleFunc("/api/identity/list", b.HandleListKeys)
	mux.HandleFunc("/api/identity/challenge", b.HandleChallenge)
}
