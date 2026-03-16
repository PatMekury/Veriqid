package parent

// VerificationMethod represents how a child's identity will be verified.
type VerificationMethod string

const (
	VerifyRemoteNotary VerificationMethod = "remote_notary"
	VerifyInPerson     VerificationMethod = "in_person"
)

// VerificationRequest represents a request to verify a child's identity.
type VerificationRequest struct {
	ChildID  int64              `json:"child_id"`
	Method   VerificationMethod `json:"method"`
	ParentID int64              `json:"parent_id"`
}

// VerificationResult is returned after a verifier approves an identity.
type VerificationResult struct {
	Success       bool   `json:"success"`
	ChildID       int64  `json:"child_id"`
	ContractIndex int    `json:"contract_index,omitempty"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// AgeBracketLabel returns a human-readable label for the age bracket code.
func AgeBracketLabel(bracket int) string {
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

// ValidateAgeBracket checks if the age bracket value is in the allowed range.
func ValidateAgeBracket(bracket int) bool {
	return bracket >= 0 && bracket <= 3
}

// NOTE: The actual on-chain registration (calling addID on the contract)
// is handled by the server in cmd/veriqid-server/main.go because it needs
// access to the Ethereum client and contract bindings. This package provides
// the data structures and validation only.
//
// In production, the verification flow would be:
//   1. Parent initiates verification via the dashboard
//   2. A remote notary or in-person verifier confirms documents
//   3. The verifier's system calls an authenticated webhook on the Veriqid server
//   4. The server calls addID() on the contract using the verifier's authorized address
//
// For the hackathon, the POST /api/parent/verify/approve endpoint simulates
// step 3 — the server approves immediately and registers on-chain.
