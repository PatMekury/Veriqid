package ve_asc

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// --- COMPREHENSIVE BENCHMARK FRAMEWORK ---
//
// Provides side-by-side comparison of:
//   Original U2SSO (CRS-ASC with Boquila ring signatures)
//   vs
//   VE-ASC (Enhanced ASC with nullifiers, epochs, attributes, Merkle trees)

// RunFullBenchmark executes the complete comparison suite.
func RunFullBenchmark() *BenchmarkReport {
	report := &BenchmarkReport{
		Timestamp:       time.Now(),
		ProtocolVersion: ProtocolVersion,
	}

	// 1. Proof size comparison across anonymity set sizes
	sizes := []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 4096, 16384, 65536, 262144, 1048576}
	for _, n := range sizes {
		report.ProofSizes = append(report.ProofSizes, benchmarkProofSize(n))
	}

	// 2. Verification cost comparison
	for _, n := range sizes {
		report.VerifyCosts = append(report.VerifyCosts, benchmarkVerifyCost(n))
	}

	// 3. Security properties comparison
	report.SecurityComparison = compareSecurityProperties()

	// 4. On-chain cost comparison
	for _, n := range []int{10, 100, 1000, 10000, 100000} {
		report.OnChainCosts = append(report.OnChainCosts, benchmarkOnChainCost(n))
	}

	// 5. Feature comparison
	report.FeatureMatrix = compareFeatures()

	// 6. Scalability analysis
	report.ScalabilityAnalysis = analyzeScalability()

	// 7. Actual timing benchmarks
	report.TimingBenchmarks = runTimingBenchmarks()

	return report
}

// BenchmarkReport contains the full comparison data.
type BenchmarkReport struct {
	Timestamp           time.Time
	ProtocolVersion     string
	ProofSizes          []ProofSizeEntry
	VerifyCosts         []VerifyCostEntry
	SecurityComparison  SecurityComparisonTable
	OnChainCosts        []OnChainCostEntry
	FeatureMatrix       FeatureComparisonTable
	ScalabilityAnalysis ScalabilityReport
	TimingBenchmarks    []TimingEntry
}

// --- PROOF SIZE BENCHMARK ---

type ProofSizeEntry struct {
	AnonymitySetSize int
	OriginalBytes    int
	VEASCBytes       int
	Improvement      string // e.g., "2.3x smaller" or "1.5x larger but with 5 more features"
}

func benchmarkProofSize(n int) ProofSizeEntry {
	logN := int(math.Ceil(math.Log2(float64(n))))
	if logN < 1 {
		logN = 1
	}

	// Original U2SSO proof:
	// SPK (33) + auth_proof (65) + ring_membership_proof (~64 * logN)
	originalSize := 33 + 65 + 64*logN

	// VE-ASC proof:
	// SPK (33) + auth_proof (65) + nullifier (32) + epoch_nullifier (32)
	// + merkle_proof (32 * logN) + attribute_proof (~128) + revocation_witness (32)
	veascSize := 33 + 65 + 32 + 32 + 32*logN + 128 + 32

	var improvement string
	if originalSize > veascSize {
		improvement = fmt.Sprintf("%.1fx smaller", float64(originalSize)/float64(veascSize))
	} else {
		extraFeatures := 5 // nullifiers, epochs, attributes, merkle, revocation
		improvement = fmt.Sprintf("%.1fx larger but +%d security features", float64(veascSize)/float64(originalSize), extraFeatures)
	}

	return ProofSizeEntry{
		AnonymitySetSize: n,
		OriginalBytes:    originalSize,
		VEASCBytes:       veascSize,
		Improvement:      improvement,
	}
}

// --- VERIFICATION COST BENCHMARK ---

type VerifyCostEntry struct {
	AnonymitySetSize    int
	OriginalOps         int
	OriginalOnChainRead int
	VEASCOps            int
	VEASCOnChainRead    int
	SpeedupFactor       float64
}

func benchmarkVerifyCost(n int) VerifyCostEntry {
	logN := int(math.Ceil(math.Log2(float64(n))))
	if logN < 1 {
		logN = 1
	}

	// Original: must fetch ALL active IDs, then verify ring
	originalOps := n + logN*2 // fetch + ring verify
	originalReads := n        // one contract call per ID (or batch)

	// VE-ASC: fetch root (1 read), verify merkle (logN hashes),
	// check nullifier (1 read), check epoch (1 read), verify attrs (constant)
	veascOps := logN + 4
	veascReads := 3

	speedup := float64(originalOps) / float64(veascOps)

	return VerifyCostEntry{
		AnonymitySetSize:    n,
		OriginalOps:         originalOps,
		OriginalOnChainRead: originalReads,
		VEASCOps:            veascOps,
		VEASCOnChainRead:    veascReads,
		SpeedupFactor:       speedup,
	}
}

// --- SECURITY PROPERTIES COMPARISON ---

type SecurityComparisonTable struct {
	Properties []SecurityProperty
}

type SecurityProperty struct {
	Name        string
	Description string
	Original    SecurityLevel
	Enhanced    SecurityLevel
}

type SecurityLevel struct {
	Supported   bool
	Mechanism   string
	Strength    string // "none", "weak", "moderate", "strong", "provable"
}

func compareSecurityProperties() SecurityComparisonTable {
	return SecurityComparisonTable{
		Properties: []SecurityProperty{
			{
				Name:        "Sybil Resistance",
				Description: "Preventing one user from registering multiple accounts per service",
				Original: SecurityLevel{
					Supported: true,
					Mechanism: "SPK uniqueness (deterministic derivation)",
					Strength:  "weak — no formal nullifier, relies on UNIQUE constraint in DB",
				},
				Enhanced: SecurityLevel{
					Supported: true,
					Mechanism: "HMAC-based nullifier with on-chain registry + DB constraint",
					Strength:  "provable — nullifier is deterministic per (MSK, service), checked on-chain",
				},
			},
			{
				Name:        "Multi-Verifier Unlinkability",
				Description: "Two services cannot determine if the same user registered on both",
				Original: SecurityLevel{
					Supported: true,
					Mechanism: "Ring signature hides which MPK in the anonymity set",
					Strength:  "moderate — ring sig provides privacy but no formal unlinkability for nullifiers (because they don't exist)",
				},
				Enhanced: SecurityLevel{
					Supported: true,
					Mechanism: "HMAC-PRF nullifiers are computationally indistinguishable across services",
					Strength:  "provable — HMAC-SHA256 PRF guarantees cross-service unlinkability of nullifiers",
				},
			},
			{
				Name:        "Temporal Sybil Resistance",
				Description: "Allowing controlled re-registration (e.g., per month) while preventing spam",
				Original: SecurityLevel{
					Supported: false,
					Mechanism: "N/A — registration is permanent",
					Strength:  "none",
				},
				Enhanced: SecurityLevel{
					Supported: true,
					Mechanism: "Epoch-scoped nullifiers: nul_epoch = HMAC(seed, service || epoch)",
					Strength:  "strong — per-epoch deterministic, configurable duration",
				},
			},
			{
				Name:        "Attribute Privacy",
				Description: "Proving properties (age >= 13) without revealing exact values",
				Original: SecurityLevel{
					Supported: false,
					Mechanism: "Age bracket stored in plaintext on-chain (uint8 ageBracket)",
					Strength:  "none — anyone reading the chain sees exact age bracket",
				},
				Enhanced: SecurityLevel{
					Supported: true,
					Mechanism: "Pedersen commitments + range proofs for selective disclosure",
					Strength:  "strong — only proves predicates, never reveals actual values",
				},
			},
			{
				Name:        "Scalable Membership Proofs",
				Description: "Efficient proofs for large anonymity sets",
				Original: SecurityLevel{
					Supported: true,
					Mechanism: "Boquila ring signatures: O(log N) proof, max N=1024",
					Strength:  "moderate — works but max 1024 identities, requires full set download",
				},
				Enhanced: SecurityLevel{
					Supported: true,
					Mechanism: "Merkle tree: O(log N) proof, O(1) root verification, max N=1M+",
					Strength:  "strong — 1000x larger anonymity sets, no full set download needed",
				},
			},
			{
				Name:        "Efficient Revocation",
				Description: "Checking identity revocation status without scanning all IDs",
				Original: SecurityLevel{
					Supported: true,
					Mechanism: "Iterate all IDs on contract, check GetState(index) for each",
					Strength:  "weak — O(N) gas cost per verification, gets expensive at scale",
				},
				Enhanced: SecurityLevel{
					Supported: true,
					Mechanism: "Sparse Merkle tree for revocation; non-membership proof O(log N)",
					Strength:  "strong — O(1) on-chain root check, O(log N) proof verification",
				},
			},
			{
				Name:        "Replay Protection",
				Description: "Preventing proof reuse across sessions",
				Original: SecurityLevel{
					Supported: true,
					Mechanism: "Server challenge (32 bytes), 5-min expiry, one-time use in DB",
					Strength:  "moderate — challenge-based but no formal binding in proof",
				},
				Enhanced: SecurityLevel{
					Supported: true,
					Mechanism: "Session nullifier = HMAC(seed, service || challenge) + challenge expiry",
					Strength:  "strong — cryptographic binding of proof to specific session",
				},
			},
		},
	}
}

// --- FEATURE COMPARISON ---

type FeatureComparisonTable struct {
	Features []FeatureEntry
}

type FeatureEntry struct {
	Feature     string
	Original    string
	Enhanced    string
	Improvement string
}

func compareFeatures() FeatureComparisonTable {
	return FeatureComparisonTable{
		Features: []FeatureEntry{
			{"Nullifier System", "Not implemented (// todo)", "3-layer HMAC nullifiers", "Critical security fix"},
			{"Epoch Support", "None", "Configurable time-bounded nullifiers", "New capability"},
			{"Attribute Proofs", "Plaintext on-chain", "Pedersen commitments + range proofs", "Privacy upgrade"},
			{"Max Anonymity Set", "1,024 (N=2, M=10)", "1,048,576 (Merkle depth=20)", "1000x increase"},
			{"On-chain Verification", "O(N) reads", "O(1) root check", "Linear → constant"},
			{"Revocation Check", "Iterate all IDs", "Sparse Merkle tree proof", "O(N) → O(log N)"},
			{"Proof Components", "2 (ring + auth)", "5 (merkle + nullifier + epoch + attr + revocation)", "3 new dimensions"},
			{"Cross-Service Unlinkability", "Implicit (different SPK)", "Formally provable (HMAC-PRF)", "Provable guarantee"},
			{"Forward Security", "None", "Epoch-based key rotation possible", "New capability"},
			{"Formal Security Proofs", "Ring sig + Boquila", "Ring sig + Boquila + HMAC-PRF + Pedersen + Merkle", "5 additional proofs"},
		},
	}
}

// --- ON-CHAIN COST ---

type OnChainCostEntry struct {
	AnonymitySetSize int
	OriginalGasRead  int // Estimated gas for reading anonymity set
	VEASCGasRead     int // Estimated gas for root verification
	GasSaving        string
}

func benchmarkOnChainCost(n int) OnChainCostEntry {
	// Original: getBatchIDs or iterate getIDs
	// Approximate gas per SLOAD: 2100 (cold), 100 (warm)
	// Each ID has 2 uint256s = 2 SLOAD = 4200 gas (cold)
	originalGas := n * 4200

	// VE-ASC: read merkle root (1 SLOAD) + check nullifier (1 SLOAD) + read revocation root (1 SLOAD)
	veascGas := 3 * 2100

	saving := fmt.Sprintf("%.0fx reduction (%d gas → %d gas)", float64(originalGas)/float64(veascGas), originalGas, veascGas)

	return OnChainCostEntry{
		AnonymitySetSize: n,
		OriginalGasRead:  originalGas,
		VEASCGasRead:     veascGas,
		GasSaving:        saving,
	}
}

// --- SCALABILITY ANALYSIS ---

type ScalabilityReport struct {
	MaxUsersOriginal int
	MaxUsersVEASC    int
	BottleneckOriginal string
	BottleneckVEASC    string
	Analysis           string
}

func analyzeScalability() ScalabilityReport {
	return ScalabilityReport{
		MaxUsersOriginal: 1024,
		MaxUsersVEASC:    1 << 20,
		BottleneckOriginal: "Ring parameter M=10 hard-caps at 2^10=1024 identities. " +
			"Increasing M requires recompiling the C library. " +
			"Each verification downloads the entire anonymity set.",
		BottleneckVEASC: "Merkle depth=20 supports 2^20=1,048,576 identities. " +
			"Can be increased to depth=32 for 4 billion identities. " +
			"Verification only needs the 32-byte root.",
		Analysis: "VE-ASC scales to production levels where the original U2SSO cannot. " +
			"For COPPA 2.0 compliance with 50M+ children in the US alone, " +
			"the original 1024-identity limit is impractical. " +
			"VE-ASC's Merkle approach with on-chain roots can handle national-scale deployment.",
	}
}

// --- TIMING BENCHMARKS ---

type TimingEntry struct {
	Operation        string
	AnonymitySetSize int
	Duration         time.Duration
	Protocol         string // "original" or "ve-asc"
}

func runTimingBenchmarks() []TimingEntry {
	entries := make([]TimingEntry, 0)
	params := Setup(24*time.Hour, 100)

	for _, n := range []int{10, 100, 1000} {
		// Benchmark Merkle tree construction
		start := time.Now()
		tree := NewMerkleTree(20)
		for i := 0; i < n; i++ {
			msk, _ := GenerateRandomMSK()
			mpk := deriveMPK(params, msk)
			tree.Insert(mpk)
		}
		entries = append(entries, TimingEntry{
			Operation:        "Build Merkle tree",
			AnonymitySetSize: n,
			Duration:         time.Since(start),
			Protocol:         "ve-asc",
		})

		// Benchmark Merkle proof generation
		start = time.Now()
		for i := 0; i < 100; i++ {
			tree.GenerateProof(0)
		}
		entries = append(entries, TimingEntry{
			Operation:        "Generate Merkle proof (×100)",
			AnonymitySetSize: n,
			Duration:         time.Since(start),
			Protocol:         "ve-asc",
		})

		// Benchmark Merkle proof verification
		proof, _ := tree.GenerateProof(0)
		root := tree.Root()
		start = time.Now()
		for i := 0; i < 1000; i++ {
			VerifyMerkleProof(proof, root)
		}
		entries = append(entries, TimingEntry{
			Operation:        "Verify Merkle proof (×1000)",
			AnonymitySetSize: n,
			Duration:         time.Since(start),
			Protocol:         "ve-asc",
		})

		// Benchmark nullifier computation
		seed := deriveNullifierSeed(make([]byte, 32))
		start = time.Now()
		for i := 0; i < 10000; i++ {
			ComputeNullifier(seed, "KidsTube")
			ComputeEpochNullifier(seed, "KidsTube", 42)
		}
		entries = append(entries, TimingEntry{
			Operation:        "Compute nullifiers (×10000)",
			AnonymitySetSize: n,
			Duration:         time.Since(start),
			Protocol:         "ve-asc",
		})

		// Benchmark attribute commitment
		start = time.Now()
		attrs := &CredentialAttributes{AgeBracket: 1, VerificationLevel: 2, ExpiryEpoch: 100}
		for i := 0; i < 100; i++ {
			msk, _ := GenerateRandomMSK()
			CommitAttributes(params, attrs, msk)
		}
		entries = append(entries, TimingEntry{
			Operation:        "Commit attributes (×100)",
			AnonymitySetSize: n,
			Duration:         time.Since(start),
			Protocol:         "ve-asc",
		})
	}

	return entries
}

// ToJSON serializes the benchmark report.
func (r *BenchmarkReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// PrintSummary outputs a human-readable summary.
func (r *BenchmarkReport) PrintSummary() string {
	s := fmt.Sprintf("=== VE-ASC Benchmark Report ===\n")
	s += fmt.Sprintf("Protocol: %s\n", r.ProtocolVersion)
	s += fmt.Sprintf("Timestamp: %s\n\n", r.Timestamp.Format(time.RFC3339))

	s += "--- Proof Size Comparison ---\n"
	s += fmt.Sprintf("%-12s  %-15s  %-15s  %s\n", "Set Size", "Original (B)", "VE-ASC (B)", "Assessment")
	for _, p := range r.ProofSizes {
		s += fmt.Sprintf("%-12d  %-15d  %-15d  %s\n", p.AnonymitySetSize, p.OriginalBytes, p.VEASCBytes, p.Improvement)
	}

	s += "\n--- Verification Cost ---\n"
	s += fmt.Sprintf("%-12s  %-15s  %-15s  %-12s\n", "Set Size", "Original Ops", "VE-ASC Ops", "Speedup")
	for _, v := range r.VerifyCosts {
		s += fmt.Sprintf("%-12d  %-15d  %-15d  %.1fx\n", v.AnonymitySetSize, v.OriginalOps, v.VEASCOps, v.SpeedupFactor)
	}

	s += "\n--- On-Chain Gas Costs ---\n"
	for _, c := range r.OnChainCosts {
		s += fmt.Sprintf("N=%d: %s\n", c.AnonymitySetSize, c.GasSaving)
	}

	s += "\n--- Scalability ---\n"
	s += fmt.Sprintf("Original max users: %d\n", r.ScalabilityAnalysis.MaxUsersOriginal)
	s += fmt.Sprintf("VE-ASC max users:   %d\n", r.ScalabilityAnalysis.MaxUsersVEASC)
	s += fmt.Sprintf("Analysis: %s\n", r.ScalabilityAnalysis.Analysis)

	s += "\n--- Timing Benchmarks ---\n"
	for _, t := range r.TimingBenchmarks {
		s += fmt.Sprintf("[N=%d] %s: %v\n", t.AnonymitySetSize, t.Operation, t.Duration)
	}

	s += "\n--- Feature Matrix ---\n"
	for _, f := range r.FeatureMatrix.Features {
		s += fmt.Sprintf("%-30s  Original: %-30s  VE-ASC: %-40s  [%s]\n", f.Feature, f.Original, f.Enhanced, f.Improvement)
	}

	return s
}
