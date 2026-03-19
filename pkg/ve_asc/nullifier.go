package ve_asc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// --- NULLIFIER SYSTEM ---
//
// The original U2SSO paper defines nullifiers as a core component of ASC
// (Section 4.2: "a prover generates a proof π along with a nullifier nul"),
// but the Veriqid implementation has "// todo no nullifiers" in server.go.
//
// VE-ASC implements proper cryptographic nullifiers with three layers:
//
// Layer 1: Service Nullifier (permanent Sybil resistance)
//   nul = HMAC-SHA256(nullifier_seed, service_name)
//   Same MSK + same service → same nullifier (deterministic)
//   Prevents double registration forever.
//
// Layer 2: Epoch Nullifier (temporal Sybil resistance)
//   epoch_nul = HMAC-SHA256(nullifier_seed, service_name || epoch_id)
//   Same MSK + same service + same epoch → same nullifier
//   Allows re-registration in new epochs (subscriptions, renewals).
//
// Layer 3: Session Nullifier (replay protection)
//   session_nul = HMAC-SHA256(nullifier_seed, service_name || challenge)
//   Unique per challenge, prevents proof replay.
//
// This is a strict improvement over the paper's single-layer nullifier.

// ComputeNullifier generates a deterministic nullifier for a service.
// This implements the formal definition: nul = Hash_srs(sk_j, v_l)
// where sk_j is the master secret key and v_l is the verifier identifier.
func ComputeNullifier(nullifierSeed []byte, serviceName string) []byte {
	mac := hmac.New(sha256.New, nullifierSeed)
	mac.Write([]byte("VE-ASC-NUL-L1:"))
	mac.Write([]byte(serviceName))
	return mac.Sum(nil)
}

// ComputeEpochNullifier generates a time-bounded nullifier.
// This is a VE-ASC innovation not present in the original paper.
// It enables use cases like:
//   - Monthly subscription renewal without creating a new identity
//   - Voting in periodic elections (one vote per epoch per service)
//   - Rate-limiting: one action per hour/day/week
func ComputeEpochNullifier(nullifierSeed []byte, serviceName string, epoch uint64) []byte {
	mac := hmac.New(sha256.New, nullifierSeed)
	mac.Write([]byte("VE-ASC-NUL-L2:"))
	mac.Write([]byte(serviceName))
	mac.Write([]byte(":"))
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, epoch)
	mac.Write(epochBytes)
	return mac.Sum(nil)
}

// ComputeSessionNullifier generates a per-challenge nullifier for replay protection.
func ComputeSessionNullifier(nullifierSeed []byte, serviceName string, challenge []byte) []byte {
	mac := hmac.New(sha256.New, nullifierSeed)
	mac.Write([]byte("VE-ASC-NUL-L3:"))
	mac.Write([]byte(serviceName))
	mac.Write([]byte(":"))
	mac.Write(challenge)
	return mac.Sum(nil)
}

// NullifierOpen allows the credential owner to prove a nullifier was generated
// by their MSK, implementing ASC.Open(crs, l, nul, Φ, sk_j).
func NullifierOpen(msk []byte, serviceName string) (nullifier []byte, proof []byte) {
	seed := deriveNullifierSeed(msk)
	nul := ComputeNullifier(seed, serviceName)

	// Proof: HMAC(msk, "OPEN" || nul) — allows owner to prove ownership
	mac := hmac.New(sha256.New, msk)
	mac.Write([]byte("VE-ASC-OPEN:"))
	mac.Write(nul)
	openProof := mac.Sum(nil)

	return nul, openProof
}

// VerifyNullifierOpen checks that a nullifier opening is valid.
func VerifyNullifierOpen(nullifier, openProof, msk []byte, serviceName string) bool {
	expectedNul, expectedProof := NullifierOpen(msk, serviceName)
	return hmac.Equal(nullifier, expectedNul) && hmac.Equal(openProof, expectedProof)
}

// --- NULLIFIER REGISTRY ---
//
// The paper states: "Each verifier maintains their own local list of nullifiers
// used to register its pseudonyms." (Section 4.2)
//
// VE-ASC improves this with a two-tier registry:
//   - On-chain: Nullifier hashes stored in smart contract (tamper-proof)
//   - Off-chain: Full nullifiers in server database (fast lookup)

// NullifierRegistry manages nullifiers for a service.
type NullifierRegistry struct {
	mu         sync.RWMutex
	nullifiers map[string]*NullifierRecord
	serviceName string
}

// NullifierRecord stores metadata about a registered nullifier.
type NullifierRecord struct {
	NullifierHex  string
	SPKHex        string // The pseudonym associated with this nullifier
	Epoch         uint64
	RegisteredAt  int64  // Unix timestamp
	OnChainIndex  int    // Index in the smart contract's nullifier array
	IsRevoked     bool
}

// NewNullifierRegistry creates a new registry for a service.
func NewNullifierRegistry(serviceName string) *NullifierRegistry {
	return &NullifierRegistry{
		nullifiers:  make(map[string]*NullifierRecord),
		serviceName: serviceName,
	}
}

// Check returns true if the nullifier has already been registered.
func (r *NullifierRegistry) Check(nullifier []byte) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.nullifiers[hex.EncodeToString(nullifier)]
	return exists
}

// Register adds a nullifier to the registry. Returns error if already exists.
func (r *NullifierRegistry) Register(nullifier []byte, spk []byte, epoch uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := hex.EncodeToString(nullifier)
	if _, exists := r.nullifiers[key]; exists {
		return fmt.Errorf("nullifier already registered — Sybil attempt for service %s", r.serviceName)
	}

	r.nullifiers[key] = &NullifierRecord{
		NullifierHex: key,
		SPKHex:       hex.EncodeToString(spk),
		Epoch:        epoch,
		RegisteredAt: getCurrentUnixTime(),
	}
	return nil
}

// CheckEpoch returns true if the nullifier has been registered in the given epoch.
func (r *NullifierRegistry) CheckEpoch(epochNullifier []byte, epoch uint64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := hex.EncodeToString(epochNullifier)
	rec, exists := r.nullifiers[key]
	return exists && rec.Epoch == epoch
}

// Revoke marks a nullifier as revoked.
func (r *NullifierRegistry) Revoke(nullifier []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := hex.EncodeToString(nullifier)
	rec, exists := r.nullifiers[key]
	if !exists {
		return fmt.Errorf("nullifier not found")
	}
	rec.IsRevoked = true
	return nil
}

// GetRecord returns the record for a nullifier, if it exists.
func (r *NullifierRegistry) GetRecord(nullifier []byte) *NullifierRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, exists := r.nullifiers[hex.EncodeToString(nullifier)]
	if !exists {
		return nil
	}
	return rec
}

// Stats returns registry statistics.
func (r *NullifierRegistry) Stats() NullifierStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := NullifierStats{
		TotalRegistered: len(r.nullifiers),
		ServiceName:     r.serviceName,
	}
	for _, rec := range r.nullifiers {
		if rec.IsRevoked {
			stats.TotalRevoked++
		}
	}
	stats.TotalActive = stats.TotalRegistered - stats.TotalRevoked
	return stats
}

// NullifierStats holds registry statistics.
type NullifierStats struct {
	ServiceName     string
	TotalRegistered int
	TotalActive     int
	TotalRevoked    int
}

// --- NULLIFIER PROPERTIES (FORMAL) ---
//
// Theorem (Nullifier Correctness):
//   For any honest prover j with MSK sk_j and any verifier l with name v_l:
//   ComputeNullifier(deriveNullifierSeed(sk_j), v_l) produces the same output
//   for all invocations → deterministic nullifier per (prover, verifier).
//
// Theorem (Nullifier Collision Resistance):
//   For two honest provers j ≠ j' with distinct MSKs sk_j ≠ sk_j':
//   Pr[ComputeNullifier(seed_j, v_l) = ComputeNullifier(seed_j', v_l)] ≤ 2^{-256}
//   → negligible collision probability.
//
// Theorem (Multi-Verifier Unlinkability):
//   For any prover j and two verifiers l ≠ l':
//   ComputeNullifier(seed_j, v_l) and ComputeNullifier(seed_j, v_l')
//   are computationally indistinguishable from random (HMAC-SHA256 PRF property).
//
// Theorem (Epoch Independence):
//   For any prover j, verifier l, and epochs e ≠ e':
//   ComputeEpochNullifier(seed_j, v_l, e) ≠ ComputeEpochNullifier(seed_j, v_l, e')
//   with overwhelming probability → fresh re-registration per epoch.

// getCurrentUnixTime returns the current unix timestamp.
func getCurrentUnixTime() int64 {
	return time.Now().Unix()
}
