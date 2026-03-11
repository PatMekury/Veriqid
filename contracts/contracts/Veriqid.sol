// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

/// @title Veriqid — Privacy-First Children's Identity Registry
/// @notice Enhanced replacement for U2SSO.sol. Fixes the owner bug,
///         adds a verifier registry, age-bracket metadata, events,
///         and batch ID retrieval for efficient ring construction.
/// @dev All identity data is a compressed secp256k1 public key (33 bytes)
///      split into uint256 (first 32 bytes) + uint8 (33rd byte).

contract Veriqid {

    // ─── Enums ──────────────────────────────────────────────────────────

    /// @notice Age bracket of the verified individual.
    ///  0 = Unknown (legacy/unset)
    ///  1 = Under 13 (full COPPA protection)
    ///  2 = 13–17    (teen protections, reduced COPPA)
    ///  3 = 18+      (adult, no COPPA restrictions)
    enum AgeBracket { Unknown, Under13, Teen, Adult }

    // ─── Structs ────────────────────────────────────────────────────────

    struct ID {
        uint256 id;            // First 32 bytes of compressed mpk
        uint    id33;          // 33rd byte of compressed mpk
        bool    active;        // Whether this identity is currently active
        address owner;         // Address that registered this ID (parent/verifier)
        AgeBracket ageBracket; // Age category for COPPA compliance
    }

    // ─── State Variables ────────────────────────────────────────────────

    /// @notice The admin address — can manage verifiers and act as fallback revoker.
    address public admin;

    /// @notice The full list of registered identities.
    ID[] public idList;

    /// @notice Total count of registered identities (matches idList.length).
    uint public nextIndex;

    /// @notice Registry of addresses authorized to register new identities.
    mapping(address => bool) public authorizedVerifiers;

    // ─── Events ─────────────────────────────────────────────────────────

    /// @notice Emitted when a new identity is registered on-chain.
    event IDRegistered(
        uint indexed index,
        address indexed owner,
        AgeBracket ageBracket
    );

    /// @notice Emitted when an identity is revoked.
    event IDRevoked(
        uint indexed index,
        address indexed revokedBy
    );

    /// @notice Emitted when a new verifier is authorized.
    event VerifierAuthorized(address indexed verifier);

    /// @notice Emitted when a verifier's authorization is removed.
    event VerifierRemoved(address indexed verifier);

    /// @notice Emitted when the admin role is transferred.
    event AdminTransferred(address indexed previousAdmin, address indexed newAdmin);

    // ─── Modifiers ──────────────────────────────────────────────────────

    /// @notice Restricts access to the contract admin.
    modifier onlyAdmin() {
        require(msg.sender == admin, "Veriqid: caller is not admin");
        _;
    }

    /// @notice Restricts addID to authorized verifiers only.
    modifier onlyVerifier() {
        require(authorizedVerifiers[msg.sender], "Veriqid: caller is not an authorized verifier");
        _;
    }

    // ─── Constructor ────────────────────────────────────────────────────

    /// @notice Deploys the contract. The deployer becomes admin AND the first
    ///         authorized verifier (so the deployer can immediately register
    ///         test identities without a separate authorization step).
    constructor() {
        admin = msg.sender;
        authorizedVerifiers[msg.sender] = true;
        emit VerifierAuthorized(msg.sender);
    }

    // ─── Admin Functions ────────────────────────────────────────────────

    /// @notice Authorize an address to register identities.
    /// @param _verifier The address to authorize (e.g., a pediatrician's wallet).
    function authorizeVerifier(address _verifier) public onlyAdmin {
        require(_verifier != address(0), "Veriqid: zero address");
        require(!authorizedVerifiers[_verifier], "Veriqid: already authorized");
        authorizedVerifiers[_verifier] = true;
        emit VerifierAuthorized(_verifier);
    }

    /// @notice Remove an address from the authorized verifier list.
    /// @param _verifier The address to de-authorize.
    function removeVerifier(address _verifier) public onlyAdmin {
        require(authorizedVerifiers[_verifier], "Veriqid: not a verifier");
        authorizedVerifiers[_verifier] = false;
        emit VerifierRemoved(_verifier);
    }

    /// @notice Transfer admin role to a new address.
    /// @param _newAdmin The address that will become the new admin.
    function transferAdmin(address _newAdmin) public onlyAdmin {
        require(_newAdmin != address(0), "Veriqid: zero address");
        emit AdminTransferred(admin, _newAdmin);
        admin = _newAdmin;
    }

    // ─── Identity Registration ──────────────────────────────────────────

    /// @notice Register a new identity on-chain.
    /// @dev    FIX: Uses msg.sender as owner (not deployer). This allows
    ///         the parent/verifier who registered the ID to revoke it later.
    /// @param _id         First 32 bytes of the compressed master public key.
    /// @param _id33       33rd byte of the compressed master public key.
    /// @param _ageBracket Age bracket (0=Unknown, 1=Under13, 2=Teen, 3=Adult).
    /// @return The index at which the new identity was stored.
    function addID(uint256 _id, uint _id33, uint8 _ageBracket) public onlyVerifier returns (uint) {
        require(_ageBracket <= uint8(type(AgeBracket).max), "Veriqid: invalid age bracket");
        idList.push(ID(_id, _id33, true, msg.sender, AgeBracket(_ageBracket)));
        uint index = nextIndex;
        nextIndex = nextIndex + 1;
        emit IDRegistered(index, msg.sender, AgeBracket(_ageBracket));
        return index;
    }

    // ─── Identity Revocation ────────────────────────────────────────────

    /// @notice Revoke an identity. Can be called by the identity's owner
    ///         (the address that originally registered it) OR by the admin
    ///         (as a safety fallback).
    /// @dev    Revokes the ENTIRE identity across all platforms — not per-platform.
    ///         This is by design: the msk derives all spks, so revoking the mpk
    ///         invalidates all service-specific keys.
    /// @param _index The index of the identity to revoke.
    function revokeID(uint _index) public {
        require(_index < nextIndex, "Veriqid: index out of bounds");
        ID storage identity = idList[_index];
        require(identity.active, "Veriqid: already revoked");
        require(
            msg.sender == identity.owner || msg.sender == admin,
            "Veriqid: only owner or admin can revoke"
        );
        identity.active = false;
        emit IDRevoked(_index, msg.sender);
    }

    // ─── Read Functions ─────────────────────────────────────────────────

    /// @notice Get the mpk components at a given index (backward compatible).
    /// @param _index Index in the idList array.
    /// @return The (id, id33) pair.
    function getIDs(uint _index) public view returns (uint256, uint) {
        require(_index < nextIndex, "Veriqid: index out of bounds");
        ID storage identity = idList[_index];
        return (identity.id, identity.id33);
    }

    /// @notice Get the active status of an identity.
    /// @param _index Index in the idList array.
    /// @return Whether the identity is active.
    function getState(uint _index) public view returns (bool) {
        require(_index < nextIndex, "Veriqid: index out of bounds");
        return idList[_index].active;
    }

    /// @notice Get the total number of registered identities.
    /// @return The count of all registered identities (active + revoked).
    function getIDSize() public view returns (uint) {
        return nextIndex;
    }

    /// @notice Find the index of an identity by its mpk components.
    /// @param _id   First 32 bytes of the compressed mpk.
    /// @param _id33 33rd byte of the compressed mpk.
    /// @return The index if found, or -1 if not found.
    function getIDIndex(uint256 _id, uint _id33) public view returns (int) {
        for (uint i = 0; i < nextIndex; i++) {
            if (idList[i].id == _id && idList[i].id33 == _id33) {
                return int(i);
            }
        }
        return -1;
    }

    /// @notice Get the age bracket of an identity.
    /// @param _index Index in the idList array.
    /// @return The age bracket enum value.
    function getAgeBracket(uint _index) public view returns (AgeBracket) {
        require(_index < nextIndex, "Veriqid: index out of bounds");
        return idList[_index].ageBracket;
    }

    /// @notice Get the owner address of an identity.
    /// @param _index Index in the idList array.
    /// @return The address that registered this identity.
    function getOwner(uint _index) public view returns (address) {
        require(_index < nextIndex, "Veriqid: index out of bounds");
        return idList[_index].owner;
    }

    // ─── Batch Retrieval ────────────────────────────────────────────────

    /// @notice Retrieve a batch of (id, id33) pairs for efficient ring construction.
    /// @dev    Returns two parallel arrays. Callers should zip them together.
    ///         Only returns active IDs to avoid including revoked identities in rings.
    /// @param _start Starting index (inclusive).
    /// @param _count Maximum number of IDs to return.
    /// @return ids    Array of first 32 bytes of each active mpk in range.
    /// @return id33s  Array of 33rd bytes of each active mpk in range.
    /// @return actives Array of active status (always true in this filtered result,
    ///                 included for caller convenience).
    function getBatchIDs(uint _start, uint _count)
        public
        view
        returns (uint256[] memory ids, uint[] memory id33s, bool[] memory actives)
    {
        require(_start < nextIndex || nextIndex == 0, "Veriqid: start out of bounds");

        // Cap the count to available IDs
        uint end = _start + _count;
        if (end > nextIndex) {
            end = nextIndex;
        }
        // First pass: count active IDs in range
        uint activeCount = 0;
        for (uint i = _start; i < end; i++) {
            if (idList[i].active) {
                activeCount++;
            }
        }

        // Allocate result arrays
        ids = new uint256[](activeCount);
        id33s = new uint[](activeCount);
        actives = new bool[](activeCount);

        // Second pass: fill arrays
        uint j = 0;
        for (uint i = _start; i < end; i++) {
            if (idList[i].active) {
                ids[j] = idList[i].id;
                id33s[j] = idList[i].id33;
                actives[j] = true;
                j++;
            }
        }

        return (ids, id33s, actives);
    }

    /// @notice Get the count of currently active (non-revoked) identities.
    /// @dev    Useful for the bridge to know the actual ring size (excluding revoked).
    /// @return The number of active identities.
    function getActiveIDCount() public view returns (uint) {
        uint count = 0;
        for (uint i = 0; i < nextIndex; i++) {
            if (idList[i].active) {
                count++;
            }
        }
        return count;
    }

    /// @notice Check if an address is an authorized verifier.
    /// @param _addr The address to check.
    /// @return Whether the address is authorized.
    function isVerifier(address _addr) public view returns (bool) {
        return authorizedVerifiers[_addr];
    }
}
