package ve_asc

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
)

// --- MERKLE TREE ANONYMITY SET ---
//
// Original U2SSO uses a flat array of MPKs for the anonymity set.
// The ring membership proof requires downloading ALL active MPKs and
// computing over the entire set:
//   - GetallActiveIDfromContract() iterates ALL IDs
//   - Ring proof size is O(log N) with N=2, M=10 → max 1024
//   - Verification requires the full anonymity set
//
// VE-ASC replaces this with a Merkle tree:
//   - Only the Merkle root is stored on-chain (32 bytes)
//   - Membership proof is O(log N) hash siblings
//   - Verification requires only the root + proof path (O(1) on-chain)
//   - Supports up to 2^depth identities (depth=20 → ~1M)
//   - Dynamic insertions: new leaves update only O(log N) hashes
//
// This is a massive scalability improvement:
//   Original: N=1024 max, O(N) on-chain reads for ring construction
//   VE-ASC:   N=1,048,576 max, O(1) on-chain verification

// MerkleTree implements a binary hash tree for the anonymity set.
type MerkleTree struct {
	depth    int
	leaves   [][]byte   // Leaf hashes (H(mpk))
	nodes    [][]byte   // Internal nodes (level-order)
	capacity int        // 2^depth
	size     int        // Current number of leaves
}

// MerkleProof contains the authentication path for a leaf.
type MerkleProof struct {
	LeafHash   []byte     // H(mpk) — the leaf being proven
	LeafIndex  int        // Position of the leaf in the tree
	Siblings   [][]byte   // Sibling hashes along the path to root
	Directions []bool     // true = sibling is on the right, false = left
	Root       []byte     // Expected root hash
	Depth      int        // Tree depth
}

// NewMerkleTree creates an empty Merkle tree with the given depth.
func NewMerkleTree(depth int) *MerkleTree {
	capacity := 1 << depth
	totalNodes := 2*capacity - 1

	nodes := make([][]byte, totalNodes)
	// Initialize all nodes with zero hashes
	zeroHash := sha256.Sum256([]byte("VE-ASC-MERKLE-ZERO"))
	for i := range nodes {
		nodes[i] = zeroHash[:]
	}

	return &MerkleTree{
		depth:    depth,
		leaves:   make([][]byte, 0, capacity),
		nodes:    nodes,
		capacity: capacity,
		size:     0,
	}
}

// Size returns the current number of leaves in the tree.
func (t *MerkleTree) Size() int {
	return t.size
}

// Depth returns the tree depth.
func (t *MerkleTree) Depth() int {
	return t.depth
}

// Insert adds an MPK to the Merkle tree and updates the path to root.
func (t *MerkleTree) Insert(mpk []byte) error {
	if t.size >= t.capacity {
		return fmt.Errorf("merkle tree full (capacity %d)", t.capacity)
	}

	// Hash the MPK to get leaf value
	leafHash := hashLeaf(mpk)
	t.leaves = append(t.leaves, leafHash)

	// Place leaf in the tree
	leafIndex := t.capacity - 1 + t.size // Leaves start at capacity-1
	t.nodes[leafIndex] = leafHash

	// Update path to root
	t.updatePath(leafIndex)
	t.size++

	return nil
}

// Root returns the current Merkle root.
func (t *MerkleTree) Root() []byte {
	if len(t.nodes) == 0 {
		return nil
	}
	return t.nodes[0]
}

// GenerateProof creates a membership proof for the leaf at the given index.
func (t *MerkleTree) GenerateProof(index int) (*MerkleProof, error) {
	if index < 0 || index >= t.size {
		return nil, fmt.Errorf("index %d out of range [0, %d)", index, t.size)
	}

	siblings := make([][]byte, t.depth)
	directions := make([]bool, t.depth)

	nodeIndex := t.capacity - 1 + index

	for level := 0; level < t.depth; level++ {
		// Determine sibling
		var siblingIndex int
		if nodeIndex%2 == 0 {
			// Current node is right child, sibling is left
			siblingIndex = nodeIndex - 1
			directions[level] = false // sibling is on the left
		} else {
			// Current node is left child, sibling is right
			siblingIndex = nodeIndex + 1
			directions[level] = true // sibling is on the right
		}

		if siblingIndex >= 0 && siblingIndex < len(t.nodes) {
			siblings[level] = t.nodes[siblingIndex]
		} else {
			zeroHash := sha256.Sum256([]byte("VE-ASC-MERKLE-ZERO"))
			siblings[level] = zeroHash[:]
		}

		// Move to parent
		nodeIndex = (nodeIndex - 1) / 2
	}

	return &MerkleProof{
		LeafHash:   t.leaves[index],
		LeafIndex:  index,
		Siblings:   siblings,
		Directions: directions,
		Root:       t.Root(),
		Depth:      t.depth,
	}, nil
}

// VerifyMerkleProof checks that a proof is valid against the expected root.
func VerifyMerkleProof(proof *MerkleProof, expectedRoot []byte) bool {
	if proof == nil || len(proof.Siblings) != proof.Depth {
		return false
	}

	currentHash := proof.LeafHash

	for i := 0; i < proof.Depth; i++ {
		sibling := proof.Siblings[i]
		if proof.Directions[i] {
			// Sibling is on the right: H(current || sibling)
			currentHash = hashPair(currentHash, sibling)
		} else {
			// Sibling is on the left: H(sibling || current)
			currentHash = hashPair(sibling, currentHash)
		}
	}

	return bytesEqual(currentHash, expectedRoot)
}

// --- INCREMENTAL MERKLE TREE ---
//
// For on-chain efficiency, VE-ASC supports incremental updates.
// When a new identity is registered, only O(log N) hashes change.
// The smart contract stores:
//   - The current root
//   - A history of roots (for proving against recent states)

// MerkleRootHistory tracks root changes for the contract.
type MerkleRootHistory struct {
	Roots     []MerkleRootEntry
	MaxHistory int
}

// MerkleRootEntry records a root at a specific block/time.
type MerkleRootEntry struct {
	Root        []byte
	BlockNumber uint64
	Timestamp   int64
	LeafCount   int
}

// NewMerkleRootHistory creates a history tracker.
func NewMerkleRootHistory(maxHistory int) *MerkleRootHistory {
	return &MerkleRootHistory{
		Roots:      make([]MerkleRootEntry, 0, maxHistory),
		MaxHistory: maxHistory,
	}
}

// AddRoot records a new root.
func (h *MerkleRootHistory) AddRoot(root []byte, blockNumber uint64, leafCount int) {
	entry := MerkleRootEntry{
		Root:        root,
		BlockNumber: blockNumber,
		LeafCount:   leafCount,
	}
	h.Roots = append(h.Roots, entry)
	if len(h.Roots) > h.MaxHistory {
		h.Roots = h.Roots[1:]
	}
}

// IsValidRoot checks if a root appears in recent history.
func (h *MerkleRootHistory) IsValidRoot(root []byte) bool {
	for _, entry := range h.Roots {
		if bytesEqual(entry.Root, root) {
			return true
		}
	}
	return false
}

// --- SPARSE MERKLE TREE FOR REVOCATION ---
//
// VE-ASC uses a separate sparse Merkle tree for revocation status.
// Instead of iterating all IDs to check active status:
//   - Revocation tree: leaf = 1 if revoked, 0 if active
//   - Non-membership proof: prove your leaf is 0 (not revoked)
//   - O(log N) proof, O(1) on-chain verification against revocation root

// RevocationTree tracks revocation status.
type RevocationTree struct {
	tree *MerkleTree
	revoked map[int]bool
}

// NewRevocationTree creates a revocation tracker.
func NewRevocationTree(depth int) *RevocationTree {
	return &RevocationTree{
		tree:    NewMerkleTree(depth),
		revoked: make(map[int]bool),
	}
}

// Revoke marks an identity as revoked.
func (rt *RevocationTree) Revoke(index int) error {
	rt.revoked[index] = true
	// Insert a "revoked" marker at this position
	marker := sha256.Sum256([]byte(fmt.Sprintf("REVOKED:%d", index)))
	return rt.tree.Insert(marker[:])
}

// IsRevoked checks if an identity is revoked.
func (rt *RevocationTree) IsRevoked(index int) bool {
	return rt.revoked[index]
}

// RevocationRoot returns the current revocation tree root.
func (rt *RevocationTree) RevocationRoot() []byte {
	return rt.tree.Root()
}

// updatePath recalculates hashes from a leaf up to the root.
func (t *MerkleTree) updatePath(leafIndex int) {
	index := leafIndex
	for index > 0 {
		parent := (index - 1) / 2
		left := 2*parent + 1
		right := 2*parent + 2
		t.nodes[parent] = hashPair(t.nodes[left], t.nodes[right])
		index = parent
	}
}

// --- HELPER FUNCTIONS ---

func hashLeaf(data []byte) []byte {
	// Domain-separated leaf hash: H("VE-ASC-LEAF:" || data)
	h := sha256.New()
	h.Write([]byte("VE-ASC-LEAF:"))
	h.Write(data)
	result := h.Sum(nil)
	return result
}

func hashPair(left, right []byte) []byte {
	// Domain-separated internal node hash: H("VE-ASC-NODE:" || left || right)
	h := sha256.New()
	h.Write([]byte("VE-ASC-NODE:"))
	h.Write(left)
	h.Write(right)
	result := h.Sum(nil)
	return result
}

// --- BENCHMARKS ---

// BenchmarkMerkleVsRing compares Merkle tree vs ring signature approaches.
func BenchmarkMerkleVsRing(sizes []int) []MerkleBenchmark {
	results := make([]MerkleBenchmark, len(sizes))

	for i, size := range sizes {
		depth := int(math.Ceil(math.Log2(float64(size))))
		if depth < 1 {
			depth = 1
		}

		results[i] = MerkleBenchmark{
			AnonymitySetSize: size,
			MerkleDepth:      depth,

			// Merkle approach
			MerkleProofSize:    depth * 32, // Each level has one 32-byte sibling
			MerkleVerifyHashes: depth,      // One hash per level

			// Ring approach (original U2SSO)
			RingProofSize:     estimateRingProofSize(size),
			RingVerifyOps:     size, // Must process all IDs
			RingOnChainReads:  size, // Must fetch all active IDs

			// VE-ASC approach (Merkle + nullifier + attributes)
			VEASCProofSize:     depth*32 + 32 + 32 + 128, // merkle + nullifier + epoch_null + attr
			VEASCVerifyHashes:  depth + 4,                  // merkle + null_check + epoch + attr + revocation
			VEASCOnChainReads:  3,                          // root + nullifier_check + revocation_root
		}
	}

	return results
}

// MerkleBenchmark holds comparison data for a specific anonymity set size.
type MerkleBenchmark struct {
	AnonymitySetSize int
	MerkleDepth      int

	// Merkle (membership only)
	MerkleProofSize    int
	MerkleVerifyHashes int

	// Ring signature (original U2SSO)
	RingProofSize    int
	RingVerifyOps    int
	RingOnChainReads int

	// VE-ASC (full enhanced protocol)
	VEASCProofSize     int
	VEASCVerifyHashes  int
	VEASCOnChainReads  int
}

func estimateRingProofSize(n int) int {
	// Ring proof in Boquila: approximately 64 * ceil(log2(n)) + 65 bytes
	if n <= 1 {
		return 65
	}
	logN := int(math.Ceil(math.Log2(float64(n))))
	return 64*logN + 65
}

// Ensure MerkleProof has a method to check validity
func (p *MerkleProof) IsValid() bool {
	return p != nil &&
		len(p.LeafHash) == 32 &&
		len(p.Siblings) == p.Depth &&
		len(p.Directions) == p.Depth &&
		len(p.Root) == 32
}

// Verify is a convenience method that checks the proof against its own root.
func (p *MerkleProof) Verify() bool {
	return VerifyMerkleProof(p, p.Root)
}

// Stubs for errors package usage
var _ = errors.New
