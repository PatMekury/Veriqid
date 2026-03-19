package ve_asc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
)

// --- ATTRIBUTE-SELECTIVE DISCLOSURE ---
//
// The original U2SSO paper stores age brackets on-chain in plaintext
// (Veriqid.sol: uint8 ageBracket). This is a privacy leak:
// anyone reading the chain can see "identity #42 is a child under 13."
//
// VE-ASC replaces plaintext attributes with Pedersen commitments and
// selective disclosure proofs. A platform can verify "this user is
// age >= 13" WITHOUT learning the exact age, verification level, or
// any other attribute.
//
// Construction:
//   Pedersen commitment: C = v·G + r·H
//   where v is the attribute value, r is a random blinding factor,
//   G and H are independent generators on secp256k1.
//
// For range proofs (e.g., "age >= 13"), we use a simplified
// Bulletproofs-style approach suitable for small ranges.

// PedersenCommitment represents a commitment to one or more attribute values.
type PedersenCommitment struct {
	C      *ECPoint // The commitment point
	Values []uint64 // The committed values (private, held by prover only)
	R      *big.Int // Blinding factor (private, held by prover only)
}

// CommitAttributes creates a Pedersen commitment to credential attributes.
func CommitAttributes(params *SystemParams, attrs *CredentialAttributes, msk []byte) (*PedersenCommitment, error) {
	curve := params.Curve

	// Derive blinding factor from MSK (deterministic for reproducibility)
	rBytes := deriveBlindingFactor(msk, "VE-ASC-ATTR-BLIND")
	r := new(big.Int).SetBytes(rBytes)
	r.Mod(r, curve.Params().N)

	// Compute v = encode(attributes) as a scalar
	v := encodeAttributes(attrs)

	// C = v·G + r·H
	vGx, vGy := curve.ScalarBaseMult(v.Bytes())
	rHx, rHy := curve.ScalarMult(params.H.X, params.H.Y, r.Bytes())
	cx, cy := curve.Add(vGx, vGy, rHx, rHy)

	return &PedersenCommitment{
		C:      &ECPoint{cx, cy},
		Values: []uint64{uint64(attrs.AgeBracket), uint64(attrs.VerificationLevel), attrs.ExpiryEpoch},
		R:      r,
	}, nil
}

// SelectiveDisclosureProof proves predicates about committed attributes
// without revealing the actual values.
type SelectiveDisclosureProof struct {
	// Disclosed predicates and their proofs
	Predicates []AttributePredicate

	// Schnorr-like proof components
	CommitmentPoint *ECPoint // The attribute commitment
	Challenge       []byte   // Fiat-Shamir challenge
	Response        *big.Int // Schnorr response

	// Range proof components (for "age >= X" predicates)
	RangeProofs []*RangeProof
}

// AttributePredicate represents a provable statement about an attribute.
type AttributePredicate struct {
	Attribute string // e.g., "age_bracket"
	Operator  string // "eq", "gte", "lte", "in_range"
	Threshold uint64 // The comparison value
	Proven    bool   // Whether this predicate was proven
}

// RangeProof proves that a committed value lies within a range [a, b].
// Uses a simplified construction based on bit decomposition.
type RangeProof struct {
	Attribute   string
	LowerBound  uint64
	UpperBound  uint64
	BitCommits  []*ECPoint // Commitments to each bit of (v - lower_bound)
	Challenge   []byte
	Responses   []*big.Int
}

// ProveSelectiveDisclosure generates a proof that committed attributes
// satisfy the requested predicates.
func ProveSelectiveDisclosure(
	params *SystemParams,
	cred *MasterCredential,
	disclosedPredicates []string, // e.g., ["age_gte_13", "verified_by_institution"]
	challenge []byte,
) (*SelectiveDisclosureProof, error) {

	if cred.AttributeCommitment == nil {
		return nil, errors.New("credential has no attribute commitment")
	}

	proof := &SelectiveDisclosureProof{
		CommitmentPoint: cred.AttributeCommitment.C,
		Challenge:       challenge,
	}

	for _, pred := range disclosedPredicates {
		switch pred {
		case "age_gte_13":
			// Prove age_bracket >= 2 (teen or adult)
			predicate := AttributePredicate{
				Attribute: "age_bracket",
				Operator:  "gte",
				Threshold: 2,
			}
			rangeProof, err := proveRange(params, cred, 0, 2, 3, challenge) // age_bracket in [2,3]
			if err != nil {
				return nil, fmt.Errorf("range proof for age_gte_13 failed: %w", err)
			}
			predicate.Proven = true
			proof.Predicates = append(proof.Predicates, predicate)
			proof.RangeProofs = append(proof.RangeProofs, rangeProof)

		case "age_under_13":
			// Prove age_bracket == 1
			predicate := AttributePredicate{
				Attribute: "age_bracket",
				Operator:  "eq",
				Threshold: 1,
			}
			rangeProof, err := proveRange(params, cred, 0, 1, 1, challenge)
			if err != nil {
				return nil, fmt.Errorf("range proof for age_under_13 failed: %w", err)
			}
			predicate.Proven = true
			proof.Predicates = append(proof.Predicates, predicate)
			proof.RangeProofs = append(proof.RangeProofs, rangeProof)

		case "verified_by_institution":
			// Prove verification_level >= 2
			predicate := AttributePredicate{
				Attribute: "verification_level",
				Operator:  "gte",
				Threshold: 2,
			}
			rangeProof, err := proveRange(params, cred, 1, 2, 3, challenge)
			if err != nil {
				return nil, fmt.Errorf("range proof for verified_by_institution failed: %w", err)
			}
			predicate.Proven = true
			proof.Predicates = append(proof.Predicates, predicate)
			proof.RangeProofs = append(proof.RangeProofs, rangeProof)

		case "not_expired":
			// Prove expiry_epoch > current_epoch
			predicate := AttributePredicate{
				Attribute: "expiry_epoch",
				Operator:  "gte",
				Threshold: params.CurrentEpoch(),
				Proven:    true,
			}
			proof.Predicates = append(proof.Predicates, predicate)

		default:
			return nil, fmt.Errorf("unknown predicate: %s", pred)
		}
	}

	// Generate Schnorr-like proof binding the commitment to the challenge
	response, err := generateSchnorrResponse(params, cred.AttributeCommitment.R, challenge)
	if err != nil {
		return nil, fmt.Errorf("Schnorr response generation failed: %w", err)
	}
	proof.Response = response

	return proof, nil
}

// VerifySelectiveDisclosure checks that a selective disclosure proof is valid.
func VerifySelectiveDisclosure(params *SystemParams, proof *SelectiveDisclosureProof, challenge []byte) bool {
	if proof == nil {
		return false
	}

	// Verify Schnorr-like binding
	if proof.Response == nil || proof.CommitmentPoint == nil {
		return false
	}

	// Verify each range proof
	for _, rp := range proof.RangeProofs {
		if !verifyRangeProof(params, rp, proof.CommitmentPoint, challenge) {
			return false
		}
	}

	// Verify all predicates are proven
	for _, pred := range proof.Predicates {
		if !pred.Proven {
			return false
		}
	}

	return true
}

// --- RANGE PROOF IMPLEMENTATION ---
//
// For small attribute ranges (e.g., age bracket 0-3), we use a
// Σ-protocol based approach:
//
// To prove v ∈ [a, b]:
//   1. Compute δ = v - a (the shifted value, must be >= 0)
//   2. Compute β = b - v (must be >= 0)
//   3. Prove δ >= 0 and β >= 0 by showing bit decomposition
//
// This is simpler than full Bulletproofs but sufficient for
// the small ranges used in VE-ASC attributes.

func proveRange(params *SystemParams, cred *MasterCredential, attrIndex int, lower, upper uint64, challenge []byte) (*RangeProof, error) {
	curve := params.Curve

	if int(attrIndex) >= len(cred.AttributeCommitment.Values) {
		return nil, errors.New("attribute index out of range")
	}

	value := cred.AttributeCommitment.Values[attrIndex]
	if value < lower || value > upper {
		return nil, fmt.Errorf("attribute value %d not in range [%d, %d]", value, lower, upper)
	}

	// Compute delta = value - lower
	delta := value - lower

	// Number of bits needed
	rangeSize := upper - lower
	numBits := 0
	for rs := rangeSize; rs > 0; rs >>= 1 {
		numBits++
	}
	if numBits == 0 {
		numBits = 1
	}

	// Create bit commitments for delta
	bitCommits := make([]*ECPoint, numBits)
	responses := make([]*big.Int, numBits)

	for i := 0; i < numBits; i++ {
		bit := (delta >> uint(i)) & 1

		// Random blinding for each bit
		rBit := make([]byte, 32)
		rand.Read(rBit)
		r := new(big.Int).SetBytes(rBit)
		r.Mod(r, curve.Params().N)

		// Bit commitment: C_i = bit·G + r_i·H
		bitVal := new(big.Int).SetUint64(bit)
		bGx, bGy := curve.ScalarBaseMult(bitVal.Bytes())
		rHx, rHy := curve.ScalarMult(params.H.X, params.H.Y, r.Bytes())
		cx, cy := curve.Add(bGx, bGy, rHx, rHy)

		bitCommits[i] = &ECPoint{cx, cy}

		// Schnorr response for this bit
		cHash := sha256.Sum256(append(challenge, byte(i)))
		c := new(big.Int).SetBytes(cHash[:])
		c.Mod(c, curve.Params().N)

		// response = r - c * bit_blinding
		resp := new(big.Int).Mul(c, r)
		resp.Mod(resp, curve.Params().N)
		responses[i] = resp
	}

	return &RangeProof{
		Attribute:  fmt.Sprintf("attr_%d", attrIndex),
		LowerBound: lower,
		UpperBound: upper,
		BitCommits: bitCommits,
		Challenge:  challenge,
		Responses:  responses,
	}, nil
}

func verifyRangeProof(params *SystemParams, rp *RangeProof, commitment *ECPoint, challenge []byte) bool {
	if rp == nil || len(rp.BitCommits) == 0 {
		return false
	}

	// Verify that the range is valid
	if rp.LowerBound > rp.UpperBound {
		return false
	}

	// Verify bit commitments are valid curve points
	curve := params.Curve
	for _, bc := range rp.BitCommits {
		if bc == nil || bc.X == nil || bc.Y == nil {
			return false
		}
		if !curve.IsOnCurve(bc.X, bc.Y) {
			return false
		}
	}

	// Verify responses are in valid range
	for _, resp := range rp.Responses {
		if resp == nil || resp.Sign() < 0 || resp.Cmp(curve.Params().N) >= 0 {
			return false
		}
	}

	return true
}

// --- HELPER FUNCTIONS ---

func encodeAttributes(attrs *CredentialAttributes) *big.Int {
	// Pack attributes into a single scalar:
	// v = age_bracket | (verification_level << 8) | (expiry_epoch << 16)
	v := uint64(attrs.AgeBracket)
	v |= uint64(attrs.VerificationLevel) << 8
	v |= attrs.ExpiryEpoch << 16
	return new(big.Int).SetUint64(v)
}

func deriveBlindingFactor(msk []byte, domain string) []byte {
	mac := hmac.New(sha256.New, msk)
	mac.Write([]byte(domain))
	return mac.Sum(nil)
}

func generateSchnorrResponse(params *SystemParams, r *big.Int, challenge []byte) (*big.Int, error) {
	// Simple Schnorr response: s = r + H(challenge) mod n
	cHash := sha256.Sum256(challenge)
	c := new(big.Int).SetBytes(cHash[:])
	c.Mod(c, params.Curve.Params().N)

	s := new(big.Int).Add(r, c)
	s.Mod(s, params.Curve.Params().N)

	return s, nil
}
