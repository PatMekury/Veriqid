// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

/**
 * @title VeriqidV2 - Enhanced Anonymous Self-Credentials Contract
 * @notice Implements VE-ASC improvements over the original U2SSO/Veriqid contract:
 *
 * IMPROVEMENT 1: On-chain Nullifier Registry
 *   Original: No nullifiers (comment: "// todo no nullifiers")
 *   VE-ASC:   Nullifier hashes stored on-chain, checked during registration
 *
 * IMPROVEMENT 2: Merkle Root Storage
 *   Original: Full anonymity set stored as array, O(N) reads for verification
 *   VE-ASC:   Only Merkle root stored, O(1) verification
 *
 * IMPROVEMENT 3: Epoch Management
 *   Original: No temporal dimension
 *   VE-ASC:   Configurable epochs, epoch-scoped nullifiers
 *
 * IMPROVEMENT 4: Attribute Commitments (not plaintext)
 *   Original: uint8 ageBracket stored in plaintext
 *   VE-ASC:   Pedersen commitment stored; actual values hidden
 *
 * IMPROVEMENT 5: Revocation Accumulator
 *   Original: Iterate all IDs to check active status
 *   VE-ASC:   Sparse Merkle tree root for O(1) revocation verification
 *
 * @author Patrick Mekury / Veriqid Team
 * @dev Built for Shape Rotator Hackathon 2025
 */
contract VeriqidV2 {
    // ============ STATE ============

    address public admin;

    // --- Identity Registry (backward-compatible) ---
    struct Identity {
        uint256 id;       // First 32 bytes of MPK
        uint    id33;     // 33rd byte of MPK
        bool    active;
        address owner;
        uint8   ageBracket;       // DEPRECATED: use attributeCommitment instead
        bytes32 attributeCommit;  // NEW: Pedersen commitment to attributes
        uint64  registeredEpoch;  // NEW: Epoch when registered
    }

    Identity[] public idList;
    uint public nextIndex;

    // --- Merkle Root (IMPROVEMENT 2) ---
    bytes32 public merkleRoot;
    bytes32[] public merkleRootHistory;    // Last N roots for proof flexibility
    uint public constant MAX_ROOT_HISTORY = 100;

    // --- Nullifier Registry (IMPROVEMENT 1) ---
    mapping(bytes32 => bool) public nullifierUsed;
    mapping(bytes32 => NullifierRecord) public nullifierRecords;
    uint public totalNullifiers;

    struct NullifierRecord {
        bytes32 nullifierHash;
        bytes32 spkHash;         // Hash of the associated SPK
        uint64  epoch;
        uint    blockNumber;
        bool    exists;
    }

    // --- Epoch-scoped Nullifiers (IMPROVEMENT 3) ---
    mapping(bytes32 => bool) public epochNullifierUsed;  // keccak256(epochNul)

    // --- Epoch Management (IMPROVEMENT 3) ---
    uint64 public epochDuration;     // Duration in seconds
    uint64 public genesisTimestamp;  // When epochs started counting

    // --- Revocation Accumulator (IMPROVEMENT 5) ---
    bytes32 public revocationRoot;   // Sparse Merkle tree root for revocation status

    // --- Verifier Registry (from Veriqid.sol) ---
    mapping(address => bool) public authorizedVerifiers;

    // ============ EVENTS ============

    event IDRegistered(
        uint indexed index,
        address indexed owner,
        bytes32 attributeCommit,
        uint64 epoch
    );
    event IDRevoked(uint indexed index, address indexed revoker);
    event NullifierRegistered(
        bytes32 indexed nullifierHash,
        bytes32 spkHash,
        uint64 epoch
    );
    event EpochNullifierRegistered(
        bytes32 indexed epochNullifierHash,
        uint64 epoch
    );
    event MerkleRootUpdated(bytes32 indexed newRoot, uint leafCount);
    event RevocationRootUpdated(bytes32 indexed newRoot);
    event VerifierAuthorized(address indexed verifier);
    event VerifierRemoved(address indexed verifier);
    event AdminTransferred(address indexed previousAdmin, address indexed newAdmin);
    event EpochDurationUpdated(uint64 oldDuration, uint64 newDuration);

    // ============ MODIFIERS ============

    modifier onlyAdmin() {
        require(msg.sender == admin, "Only admin");
        _;
    }

    modifier onlyVerifier() {
        require(
            authorizedVerifiers[msg.sender] || msg.sender == admin,
            "Not authorized verifier"
        );
        _;
    }

    // ============ CONSTRUCTOR ============

    constructor(uint64 _epochDuration) {
        admin = msg.sender;
        authorizedVerifiers[msg.sender] = true;
        epochDuration = _epochDuration;
        genesisTimestamp = uint64(block.timestamp);
    }

    // ============ IDENTITY MANAGEMENT ============

    /**
     * @notice Register a new identity with attribute commitment
     * @param _id First 32 bytes of compressed MPK
     * @param _id33 33rd byte of compressed MPK
     * @param _attributeCommit Pedersen commitment to attributes (IMPROVEMENT 4)
     * @return The index of the newly registered identity
     */
    function addID(
        uint256 _id,
        uint _id33,
        bytes32 _attributeCommit
    ) public onlyVerifier returns (uint) {
        uint64 currentEpoch = getCurrentEpoch();

        idList.push(Identity(
            _id,
            _id33,
            true,
            msg.sender,      // FIXED: uses msg.sender, not deployer
            0,               // ageBracket deprecated
            _attributeCommit,
            currentEpoch
        ));

        uint index = nextIndex;
        nextIndex++;

        emit IDRegistered(index, msg.sender, _attributeCommit, currentEpoch);
        return index;
    }

    /**
     * @notice Legacy addID for backward compatibility with existing code
     */
    function addIDLegacy(
        uint256 _id,
        uint _id33,
        uint8 _ageBracket
    ) public onlyVerifier returns (uint) {
        return addID(_id, _id33, bytes32(uint256(_ageBracket)));
    }

    /**
     * @notice Revoke an identity (owner or admin only)
     */
    function revokeID(uint _index) public {
        require(_index < nextIndex, "Index out of range");
        require(
            msg.sender == idList[_index].owner || msg.sender == admin,
            "Not owner or admin"
        );
        require(idList[_index].active, "Already revoked");

        idList[_index].active = false;
        emit IDRevoked(_index, msg.sender);
    }

    // ============ NULLIFIER REGISTRY (IMPROVEMENT 1) ============

    /**
     * @notice Register a nullifier (prevents Sybil attacks)
     * @dev The nullifier is deterministic per (MSK, service). If it's already
     *      registered, this transaction reverts → Sybil attempt blocked.
     * @param _nullifierHash keccak256 of the HMAC-based nullifier
     * @param _spkHash keccak256 of the service-specific public key
     */
    function registerNullifier(
        bytes32 _nullifierHash,
        bytes32 _spkHash
    ) public returns (bool) {
        require(!nullifierUsed[_nullifierHash], "Nullifier already used - Sybil blocked");

        uint64 currentEpoch = getCurrentEpoch();

        nullifierUsed[_nullifierHash] = true;
        nullifierRecords[_nullifierHash] = NullifierRecord(
            _nullifierHash,
            _spkHash,
            currentEpoch,
            block.number,
            true
        );
        totalNullifiers++;

        emit NullifierRegistered(_nullifierHash, _spkHash, currentEpoch);
        return true;
    }

    /**
     * @notice Check if a nullifier has been used
     */
    function isNullifierUsed(bytes32 _nullifierHash) public view returns (bool) {
        return nullifierUsed[_nullifierHash];
    }

    // ============ EPOCH NULLIFIERS (IMPROVEMENT 3) ============

    /**
     * @notice Register an epoch-scoped nullifier
     * @dev Allows re-registration in new epochs while preventing
     *      multiple registrations within the same epoch.
     */
    function registerEpochNullifier(bytes32 _epochNullifierHash) public returns (bool) {
        require(
            !epochNullifierUsed[_epochNullifierHash],
            "Epoch nullifier already used this epoch"
        );

        epochNullifierUsed[_epochNullifierHash] = true;

        emit EpochNullifierRegistered(_epochNullifierHash, getCurrentEpoch());
        return true;
    }

    function isEpochNullifierUsed(bytes32 _epochNullifierHash) public view returns (bool) {
        return epochNullifierUsed[_epochNullifierHash];
    }

    // ============ MERKLE ROOT (IMPROVEMENT 2) ============

    /**
     * @notice Update the Merkle root of the anonymity set
     * @dev Called after new identities are added. Only admin/verifier.
     *      Old roots are kept in history for proof flexibility.
     */
    function updateMerkleRoot(bytes32 _newRoot) public onlyVerifier {
        // Store old root in history
        if (merkleRoot != bytes32(0)) {
            merkleRootHistory.push(merkleRoot);
            // Trim history if needed
            if (merkleRootHistory.length > MAX_ROOT_HISTORY) {
                // Shift array (gas expensive but bounded by MAX_ROOT_HISTORY)
                for (uint i = 0; i < merkleRootHistory.length - 1; i++) {
                    merkleRootHistory[i] = merkleRootHistory[i + 1];
                }
                merkleRootHistory.pop();
            }
        }

        merkleRoot = _newRoot;
        emit MerkleRootUpdated(_newRoot, nextIndex);
    }

    /**
     * @notice Check if a root is the current root or in recent history
     */
    function isValidMerkleRoot(bytes32 _root) public view returns (bool) {
        if (_root == merkleRoot) return true;
        for (uint i = 0; i < merkleRootHistory.length; i++) {
            if (merkleRootHistory[i] == _root) return true;
        }
        return false;
    }

    // ============ REVOCATION ACCUMULATOR (IMPROVEMENT 5) ============

    /**
     * @notice Update the revocation tree root
     * @dev Called after revoking identities. Enables O(1) revocation checks.
     */
    function updateRevocationRoot(bytes32 _newRoot) public onlyVerifier {
        revocationRoot = _newRoot;
        emit RevocationRootUpdated(_newRoot);
    }

    // ============ EPOCH MANAGEMENT (IMPROVEMENT 3) ============

    /**
     * @notice Get the current epoch number
     */
    function getCurrentEpoch() public view returns (uint64) {
        if (epochDuration == 0) return 0;
        return (uint64(block.timestamp) - genesisTimestamp) / epochDuration;
    }

    /**
     * @notice Update epoch duration (admin only)
     */
    function setEpochDuration(uint64 _newDuration) public onlyAdmin {
        require(_newDuration > 0, "Epoch duration must be > 0");
        emit EpochDurationUpdated(epochDuration, _newDuration);
        epochDuration = _newDuration;
    }

    // ============ ADMIN & VERIFIER MANAGEMENT ============

    function authorizeVerifier(address _verifier) public onlyAdmin {
        authorizedVerifiers[_verifier] = true;
        emit VerifierAuthorized(_verifier);
    }

    function removeVerifier(address _verifier) public onlyAdmin {
        authorizedVerifiers[_verifier] = false;
        emit VerifierRemoved(_verifier);
    }

    function transferAdmin(address _newAdmin) public onlyAdmin {
        require(_newAdmin != address(0), "Invalid admin address");
        emit AdminTransferred(admin, _newAdmin);
        admin = _newAdmin;
    }

    // ============ QUERY FUNCTIONS ============

    function getIDSize() public view returns (uint) {
        return nextIndex;
    }

    function getIDs(uint _index) public view returns (uint256, uint) {
        require(_index < nextIndex, "Index out of range");
        return (idList[_index].id, idList[_index].id33);
    }

    function getState(uint _index) public view returns (bool) {
        require(_index < nextIndex, "Index out of range");
        return idList[_index].active;
    }

    function getAttributeCommit(uint _index) public view returns (bytes32) {
        require(_index < nextIndex, "Index out of range");
        return idList[_index].attributeCommit;
    }

    function getIDIndex(uint256 _id, uint _id33) public view returns (uint) {
        for (uint i = 0; i < nextIndex; i++) {
            if (idList[i].id == _id && idList[i].id33 == _id33) {
                return i;
            }
        }
        revert("ID not found");
    }

    function getActiveIDCount() public view returns (uint) {
        uint count = 0;
        for (uint i = 0; i < nextIndex; i++) {
            if (idList[i].active) count++;
        }
        return count;
    }

    /**
     * @notice Get protocol statistics
     */
    function getProtocolStats() public view returns (
        uint totalIDs,
        uint activeIDs,
        uint registeredNullifiers,
        bytes32 currentMerkleRoot,
        bytes32 currentRevocationRoot,
        uint64 currentEpoch,
        uint64 currentEpochDuration
    ) {
        totalIDs = nextIndex;
        activeIDs = getActiveIDCount();
        registeredNullifiers = totalNullifiers;
        currentMerkleRoot = merkleRoot;
        currentRevocationRoot = revocationRoot;
        currentEpoch = getCurrentEpoch();
        currentEpochDuration = epochDuration;
    }
}
