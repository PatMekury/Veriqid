# Veriqid Phase 3: Enhanced Smart Contract — Complete Detailed Guide

**Time estimate: ~3 hours**
**Goal: Create `contracts/Veriqid.sol` — an enhanced smart contract that fixes the owner bug, adds a verifier registry, emits events, supports age-bracket metadata, and provides batch ID retrieval. Then regenerate Go bindings and update all callers.**

---

> **IMPORTANT — Restarting Servers for Future Phases**
>
> If you shut down Ganache, the bridge, or the server at any point (between sessions, between phases, etc.), you will need to restart them before doing any work that touches the blockchain or bridge:
>
> 1. **Start Ganache** (Terminal 1):
>    ```bash
>    ganache --port 7545
>    ```
> 2. **Redeploy the contract** (new terminal — Ganache is ephemeral, all data is lost on restart):
>    ```bash
>    cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
>    truffle migrate --reset --network development
>    ```
>    Copy the new contract address from the output.
> 3. **Start the bridge** (Terminal 2):
>    ```bash
>    cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
>    go build -o bridge-server ./cmd/bridge/
>    ./bridge-server -contract 0x<NEW_CONTRACT_ADDRESS> -client http://127.0.0.1:7545
>    ```
> 4. **Start the server** (Terminal 3, if needed):
>    ```bash
>    cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
>    go run ./cmd/server -contract 0x<NEW_CONTRACT_ADDRESS>
>    ```
> 5. **Re-create identities** — old key files exist on disk but are not registered on the new contract. Either create fresh keys via the bridge API or re-run the test script.
>
> You will also need to use one of the Ganache private keys (without the `0x` prefix) for any operations that send Ethereum transactions (like creating identities or authorizing verifiers).

---

## BEFORE YOU START: Understanding What You're Building

In Phases 1–2, the smart contract used was `U2SSO.sol` — a minimal proof-of-concept with a critical bug and no access control. Phase 3 creates `Veriqid.sol`, an enhanced contract that makes the system production-ready.

### What's Wrong with U2SSO.sol

Here's the current `addID()` function:

```solidity
function addID (uint256 _id, uint _id33) public returns (uint) {
    idList.push(ID(_id, _id33, true, _owner));   // BUG: _owner = deployer, NOT msg.sender
    nextIndex = nextIndex + 1;
    return nextIndex - 1;
}
```

**The bug:** `_owner` is set in the `constructor()` to `msg.sender` — the address that deployed the contract. Every ID added via `addID()` gets `_owner` as its owner. This means:

- **Only the contract deployer can revoke ANY identity** — not the parent who registered it
- **Parents cannot revoke their own children's identities** — the "Parent Revocable" claim is broken
- **No access control** — anyone can call `addID()` and register arbitrary master public keys

### What's Also Missing

1. **Verifier registry** — In production, only trusted verifiers (pediatricians, schools, government portals) should be able to register identities. Currently anyone can.
2. **Events** — No Ethereum events are emitted, so the parent dashboard (Phase 6) has no way to listen for registrations or revocations without polling.
3. **Age-bracket metadata** — Platforms need to know "this is a verified child under 13" vs "this is a verified teen 13-17" to apply different COPPA rules. The struct has no field for this.
4. **Batch retrieval** — `getIDs()` returns one ID at a time. Building the ring for membership proofs requires fetching ALL active IDs, which currently needs N separate contract calls. A batch function is essential for gas-efficient ring construction.

### What Veriqid.sol Adds

| Feature | U2SSO.sol | Veriqid.sol |
|---------|-----------|-------------|
| **Owner of ID** | Contract deployer (bug) | `msg.sender` (the parent/verifier who called `addID`) |
| **Access control** | None (anyone can add) | Only authorized verifiers can call `addID()` |
| **Events** | None | `IDRegistered`, `IDRevoked`, `VerifierAuthorized`, `VerifierRemoved` |
| **Age bracket** | Not stored | `uint8 ageBracket` (0=unknown, 1=under-13, 2=13-17, 3=18+) |
| **Batch retrieval** | One at a time | `getBatchIDs(start, count)` returns multiple IDs in one call |
| **Revocation** | Only deployer can revoke | Owner of each ID can revoke, OR contract admin |
| **Admin role** | Implicit (deployer) | Explicit `admin` with transfer capability |

### Backward Compatibility

`Veriqid.sol` is **NOT an extension** of `U2SSO.sol` — it's a **replacement**. The function signatures change (e.g., `addID` gains new parameters), and the struct gains a new field. However, the existing Go code in `u2ssolib.go` calls the contract through **auto-generated Go bindings** (`U2SSO.go`), so we need to:

1. Write the new Solidity contract
2. Compile it
3. Regenerate the Go bindings using `abigen`
4. Update the bridge and server code to use the new function signatures

---

## PREREQUISITES

Before starting Phase 3, you must have Phases 1 and 2 fully complete:

```
[x] libsecp256k1 built and installed with --enable-module-ringcip
[x] go build ./cmd/client/ succeeds (CGO works)
[x] go build ./cmd/server/ succeeds
[x] bridge/bridge.go compiles (go build -o bridge-server ./cmd/bridge/)
[x] Ganache running on port 7545
[x] U2SSO.sol deployed (Phase 1)
[x] Bridge API tested (Phase 2)
```

You'll also need:
- **Truffle** installed (from Phase 1)
- **`abigen`** — the go-ethereum ABI binding generator (we'll install it in Step 5)
- A Ganache private key handy

---

## STEP 1: Review the Current U2SSO.sol Contract

**Time: ~10 minutes (reading, no coding)**

### 1.1 Open the existing contract

```bash
cat /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts/U2SSO.sol
```

The full contract is 62 lines:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

contract U2SSO {
    struct ID {
        uint256 id;
        uint id33;
        bool active;
        address owner;
    }

    address private _owner;
    constructor() {
        _owner = msg.sender;
    }

    ID[] public idList;
    uint nextIndex;

    function addID (uint256 _id, uint _id33) public returns (uint) {
        idList.push(ID(_id, _id33, true, _owner));  // BUG: always deployer
        nextIndex = nextIndex + 1;
        return nextIndex - 1;
    }

    function revokeID (uint _index) public {
        ID storage id = idList[_index];
        if (_owner == id.owner) {      // BUG: only deployer matches
            id.active = false;
        }
    }

    function getIDs (uint _index) public view returns (uint256, uint) { ... }
    function getState (uint _index) public view returns (bool) { ... }
    function getIDSize () public view returns (uint) { ... }
    function getIDIndex (uint256 _id, uint _id33) public view returns (int) { ... }
}
```

### 1.2 Key observations about the existing code

1. **`_owner`** is a single `address` set at deploy time — it's the deployer, NOT the identity owner
2. **`addID()`** takes `(uint256 _id, uint _id33)` — these are the two parts of the 33-byte compressed mpk split into a 256-bit integer and a remaining byte
3. **`revokeID()`** checks `_owner == id.owner` — but since ALL IDs have `_owner` (deployer) as owner, only the deployer can revoke anything
4. **No events** — the `push()` and `active = false` happen silently
5. **`getIDIndex()`** does a linear scan — O(n). This is fine for small rings but will be a gas issue at scale
6. **`idList`** is a simple `ID[]` array — no mapping for O(1) lookup

### 1.3 What the Go code depends on

The Go bindings (`U2SSO.go`) expose these Solidity functions as Go methods:

- `AddID(_id *big.Int, _id33 *big.Int)` → transact
- `RevokeID(_index *big.Int)` → transact
- `GetIDs(_index *big.Int)` → call, returns `(*big.Int, *big.Int)`
- `GetState(_index *big.Int)` → call, returns `bool`
- `GetIDSize()` → call, returns `*big.Int`
- `GetIDIndex(_id *big.Int, _id33 *big.Int)` → call, returns `*big.Int`
- `IdList(arg0 *big.Int)` → call, returns struct

The bridge (`bridge.go`) and library (`u2ssolib.go`) call these Go methods. When we change the Solidity signatures, the Go bindings change, and all callers must be updated.

---

## STEP 2: Write contracts/Veriqid.sol

**Time: ~30 minutes**

### 2.1 Navigate to the contracts directory

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
```

### 2.2 Create the new contract file

Create `Veriqid.sol` alongside the existing `U2SSO.sol`:

```bash
nano Veriqid.sol
```

Or use your preferred editor. Paste the following complete contract:

```solidity
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
        uint256 id;          // First 32 bytes of compressed mpk
        uint    id33;        // 33rd byte of compressed mpk
        bool    active;      // Whether this identity is currently active
        address owner;       // Address that registered this ID (parent/verifier)
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
    /// @param _id        First 32 bytes of the compressed master public key.
    /// @param _id33      33rd byte of the compressed master public key.
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
```

### 2.3 Save the file and verify

```bash
ls -la Veriqid.sol
# Should show the new file

head -5 Veriqid.sol
# Should show: // SPDX-License-Identifier: MIT
```

### 2.4 Understanding the key design decisions

**Why `addID` gains a third parameter (`_ageBracket`):**

The existing Go code in `u2ssolib.go` calls `AddID(opts, id, id33)` with two `*big.Int` arguments. The new contract adds a `uint8 _ageBracket` parameter. This means:
- The Solidity function signature changes: `addID(uint256,uint,uint8)` vs `addID(uint256,uint)`
- The auto-generated Go method changes: `AddID(opts, id, id33, ageBracket)`
- All callers (u2ssolib.go, bridge.go, client main.go) must be updated to pass the new argument

**Why the constructor auto-authorizes the deployer:**

For testing, the deployer needs to immediately create identities. Without auto-authorization, you'd need a separate `authorizeVerifier(myAddress)` transaction before your first `addID()` call. The deployer being both admin and first verifier streamlines the development workflow.

**Why `revokeID` allows both owner AND admin:**

- **Owner** (parent/verifier) should be able to revoke the identity they registered — this is the primary use case for parent revocation
- **Admin** acts as a safety fallback — if a verifier is compromised or the parent loses access, the platform admin can still revoke

**Why `getBatchIDs` filters out revoked IDs:**

Ring membership proofs should only include ACTIVE identities. Including revoked IDs would mean the ring contains "dead" entries that no one can prove membership for, wasting proof computation and inflating the ring size.

---

## STEP 3: Update the Truffle Migration

**Time: ~5 minutes**

### 3.1 Create a new migration file for Veriqid

Truffle runs migrations in numeric order. Create a second migration for the new contract:

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
```

Create `migrations/2_deploy_veriqid.js`:

```bash
cat > migrations/2_deploy_veriqid.js << 'EOF'
const Veriqid = artifacts.require("Veriqid.sol");

module.exports = function(deployer) {
  deployer.deploy(Veriqid);
};
EOF
```

### 3.2 Copy Veriqid.sol into the Truffle contracts subfolder

Remember from Phase 1: Truffle looks for Solidity files in `contracts/contracts/`:

```bash
cp Veriqid.sol contracts/
```

### 3.3 Verify the directory structure

```bash
ls contracts/
# Expected: U2SSO.sol  Veriqid.sol

ls migrations/
# Expected: 1_deploy_contracts.js  2_deploy_veriqid.js
```

---

## STEP 4: Compile and Deploy Veriqid.sol

**Time: ~10 minutes**

### 4.1 Make sure Ganache is running

In a separate terminal:

```bash
ganache --port 7545
```

### 4.2 Compile the contract

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts

truffle compile
```

**Expected output:**

```
Compiling your contracts...
===========================
> Compiling ./contracts/U2SSO.sol
> Compiling ./contracts/Veriqid.sol
> Artifacts written to ./build/contracts
> Compiled successfully using:
   - solc: 0.8.13+commit.abaa5c0e.Emscripten.clang
```

**If it fails with a compile error:** Read the error message. Common issues:
- Syntax error → check for missing semicolons or braces in Veriqid.sol
- `type(AgeBracket).max` not recognized → make sure you're using Solidity 0.8.13+ (check `truffle-config.js`)

### 4.3 Deploy to Ganache

```bash
truffle migrate --reset --network development
```

**Why `--reset`?** This resets ALL migrations, redeploying both `U2SSO.sol` and `Veriqid.sol` from scratch. Ganache is ephemeral anyway, so there's no data to preserve.

**Expected output:**

```
Starting migrations...
======================

1_deploy_contracts.js
=====================
   Deploying 'U2SSO'
   ------------------
   > transaction hash:    0x...
   > contract address:    0x...   <-- U2SSO address (you can ignore this)
   > ...

2_deploy_veriqid.js
===================
   Deploying 'Veriqid'
   -------------------
   > transaction hash:    0x...
   > contract address:    0xYourVeriqidAddress  <-- SAVE THIS!
   > block number:        3
   > gas used:            ...
   > ...

Summary
=======
> Total deployments:   2
> Final cost:          ...
```

### 4.4 SAVE THE VERIQID CONTRACT ADDRESS

Copy the **Veriqid** contract address (from the `2_deploy_veriqid.js` section, NOT the U2SSO one). You'll use this for all subsequent commands.

### 4.5 Verify deployment via Truffle console (optional)

```bash
truffle console --network development
```

Inside the console:

```javascript
let v = await Veriqid.deployed()
let admin = await v.admin()
console.log("Admin:", admin)
// Should show the first Ganache account (the deployer)

let isVerifier = await v.isVerifier(admin)
console.log("Admin is verifier:", isVerifier)
// Should show: true

let size = await v.getIDSize()
console.log("ID count:", size.toString())
// Should show: 0

.exit
```

### 4.6 Troubleshooting deployment

| Error | Fix |
|-------|-----|
| `Could not connect to your Ethereum client` | Make sure Ganache is running on port 7545 |
| `Could not find artifacts for Veriqid from any sources` | Veriqid.sol is not in the `contracts/contracts/` subfolder. Run `cp Veriqid.sol contracts/` |
| `CompileError: TypeError: ...` | Syntax or type error in Veriqid.sol — read the error message carefully |
| `Deployer does not have enough funds` | Restart Ganache — each fresh start gives 1000 ETH per account |

---

## STEP 5: Install abigen and Regenerate Go Bindings

**Time: ~15 minutes**

### 5.1 Understanding why we need abigen

The file `pkg/u2sso/U2SSO.go` is NOT hand-written — it was auto-generated by `abigen`, a tool from the go-ethereum project. It reads the contract's **ABI** (Application Binary Interface) and generates type-safe Go methods.

When we change the Solidity contract (new function signatures, new events, new fields), we must regenerate this file so the Go code matches the new contract.

### 5.2 Install abigen

```bash
go install github.com/ethereum/go-ethereum/cmd/abigen@latest
```

**What this does:** Downloads and compiles the `abigen` binary from the go-ethereum source, placing it in `$HOME/go/bin/`. Since we added `$HOME/go/bin` to PATH in Phase 1, it should be immediately available.

### 5.3 Verify installation

```bash
abigen --version
# Expected: abigen version 1.x.x (exact version depends on go-ethereum release)
```

**If it says `abigen: command not found`:**

```bash
# Make sure Go bin is in PATH
export PATH=$PATH:$HOME/go/bin
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc

# Try again
abigen --version
```

### 5.4 Locate the compiled ABI and bytecode

Truffle puts the compiled artifacts in `build/contracts/`:

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts

ls build/contracts/
# Expected: U2SSO.json  Veriqid.json  (and possibly Migrations.json)
```

The `Veriqid.json` file contains the ABI, bytecode, and other metadata.

### 5.5 Extract the ABI into a separate file

`abigen` can read the ABI from the Truffle JSON artifact, but it's cleaner to extract just the ABI:

```bash
# Extract ABI from Truffle artifact
python3 -c "
import json
with open('build/contracts/Veriqid.json') as f:
    data = json.load(f)
with open('build/contracts/Veriqid.abi', 'w') as f:
    json.dump(data['abi'], f)
print('ABI extracted successfully')
print(f'Functions: {len([x for x in data[\"abi\"] if x.get(\"type\") == \"function\"])}')
print(f'Events: {len([x for x in data[\"abi\"] if x.get(\"type\") == \"event\"])}')
"
```

**Expected output:**

```
ABI extracted successfully
Functions: 18
Events: 5
```

The 18 functions are: `addID`, `admin`, `authorizeVerifier`, `authorizedVerifiers`, `getActiveIDCount`, `getAgeBracket`, `getBatchIDs`, `getIDIndex`, `getIDSize`, `getIDs`, `getOwner`, `getState`, `idList`, `isVerifier`, `nextIndex`, `removeVerifier`, `revokeID`, `transferAdmin`. (Some of these are auto-generated public getter functions for state variables like `admin`, `authorizedVerifiers`, `idList`, and `nextIndex`.)

The 5 events are: `AdminTransferred`, `IDRegistered`, `IDRevoked`, `VerifierAuthorized`, `VerifierRemoved`.

### 5.6 Generate the new Go bindings

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

abigen \
  --abi contracts/build/contracts/Veriqid.abi \
  --pkg u2sso \
  --type Veriqid \
  --out pkg/u2sso/Veriqid.go
```

**What the flags mean:**

- `--abi` — path to the ABI JSON file
- `--pkg u2sso` — Go package name (must match the existing package in `pkg/u2sso/`)
- `--type Veriqid` — the Go struct name for the contract binding (produces `Veriqid`, `VeriqidCaller`, `VeriqidTransactor`, etc.)
- `--out` — output file path

### 5.7 Verify the generated file

```bash
head -20 pkg/u2sso/Veriqid.go
# Should start with:
# // Code generated - DO NOT EDIT.
# // This file is a generated binding and any manual changes will be lost.
# package u2sso

# Check that key functions exist
grep "func.*AddID" pkg/u2sso/Veriqid.go
# Expected: func (_Veriqid *VeriqidTransactor) AddID(opts *bind.TransactOpts, _id *big.Int, _id33 *big.Int, _ageBracket uint8) (*types.Transaction, error)

grep "func.*RevokeID" pkg/u2sso/Veriqid.go
# Expected: func (_Veriqid *VeriqidTransactor) RevokeID(opts *bind.TransactOpts, _index *big.Int) (*types.Transaction, error)

grep "func.*GetBatchIDs" pkg/u2sso/Veriqid.go
# Expected: func (_Veriqid *VeriqidCaller) GetBatchIDs(opts *bind.CallOpts, _start *big.Int, _count *big.Int) (struct { ... }, error)

grep "func.*AuthorizeVerifier" pkg/u2sso/Veriqid.go
# Expected: func (_Veriqid *VeriqidTransactor) AuthorizeVerifier(opts *bind.TransactOpts, _verifier common.Address) (*types.Transaction, error)
```

### 5.8 Keep the old U2SSO.go

**Do NOT delete `pkg/u2sso/U2SSO.go`** yet. The existing `u2ssolib.go` still references `U2sso` (the old type name). We'll update the references in Step 6 and Step 7. Having both files in the same package is fine — they define different types (`U2sso` vs `Veriqid`).

### 5.9 Verify the package compiles

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

go build ./pkg/u2sso/
```

**Expected:** No output (success). If there are compilation errors, it's likely a naming collision between the old and new bindings. Check that `--type Veriqid` was used (not `--type U2sso`).

---

## STEP 6: Update u2ssolib.go to Support Veriqid Contract

**Time: ~30 minutes**

### 6.1 Understanding what needs to change

The core crypto functions (`CreatePasskey`, `LoadPasskey`, `CreateID`, `RegistrationProof`, `AuthProof`, `RegistrationVerify`, `AuthVerify`, `CreateChallenge`) do NOT change — they're pure cryptography that doesn't touch the contract.

What DOES change are the contract interaction functions:

| Function | Old Signature | New Signature |
|----------|--------------|---------------|
| `AddIDstoIdR` | Takes `*U2sso` instance | Takes `*Veriqid` instance + `ageBracket uint8` |
| `GetIDfromContract` | Takes `*U2sso` | Takes `*Veriqid` |
| `GetIDIndexfromContract` | Takes `*U2sso` | Takes `*Veriqid` |
| `GetallActiveIDfromContract` | Takes `*U2sso` | Takes `*Veriqid` |

### 6.2 Update the contract interaction functions in u2ssolib.go

Open `pkg/u2sso/u2ssolib.go` and make these changes:

**Change A: Update `AddIDstoIdR` to use Veriqid and accept ageBracket**

Find:
```go
func AddIDstoIdR(client *ethclient.Client, sk string, inst *U2sso, id []byte) (int64, error) {
```

Replace with:
```go
func AddIDstoIdR(client *ethclient.Client, sk string, inst *Veriqid, id []byte, ageBracket uint8) (int64, error) {
```

Then, inside the function body, find the `AddID` call:
```go
tx, err := inst.AddID(auth, idBig, id33Big)
```

Replace with:
```go
tx, err := inst.AddID(auth, idBig, id33Big, ageBracket)
```

**Change B: Update `GetIDfromContract` to use Veriqid**

Find:
```go
func GetIDfromContract(inst *U2sso) (int64, error) {
```

Replace with:
```go
func GetIDfromContract(inst *Veriqid) (int64, error) {
```

**Change C: Update `GetIDIndexfromContract` to use Veriqid**

Find:
```go
func GetIDIndexfromContract(inst *U2sso, id []byte) (int64, error) {
```

Replace with:
```go
func GetIDIndexfromContract(inst *Veriqid, id []byte) (int64, error) {
```

**Change D: Update `GetallActiveIDfromContract` to use Veriqid**

Find:
```go
func GetallActiveIDfromContract(inst *U2sso) ([][]byte, error) {
```

Replace with:
```go
func GetallActiveIDfromContract(inst *Veriqid) ([][]byte, error) {
```

### 6.3 Add a new helper to instantiate the Veriqid contract

The old code used `NewU2sso(address, client)` to create a contract instance. The bridge and CLI will now need `NewVeriqid(address, client)`. Check that `Veriqid.go` contains a `NewVeriqid` function (it should — `abigen` generates it automatically).

```bash
grep "func NewVeriqid" pkg/u2sso/Veriqid.go
# Expected: func NewVeriqid(address common.Address, backend bind.ContractBackend) (*Veriqid, error)
```

### 6.4 Verify u2ssolib.go compiles

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go build ./pkg/u2sso/
```

If it fails with `undefined: U2sso` errors, you may still have functions referencing the old type. Check:

```bash
grep "U2sso" pkg/u2sso/u2ssolib.go
```

Any remaining references to `*U2sso` should be changed to `*Veriqid` (except in comments).

---

## STEP 7: Update the Bridge, Client, and Server

**Time: ~30 minutes**

### 7.1 Update bridge/bridge.go

The bridge's `connectToContract` helper currently uses `NewU2sso`. Change it to `NewVeriqid`:

**Change A: Update the connectToContract helper**

Find:
```go
func connectToContract(rpcURL, contractAddr string) (*ethclient.Client, *u2sso.U2sso, error) {
```

Replace with:
```go
func connectToContract(rpcURL, contractAddr string) (*ethclient.Client, *u2sso.Veriqid, error) {
```

And inside the function, find:
```go
instance, err := u2sso.NewU2sso(address, client)
```

Replace with:
```go
instance, err := u2sso.NewVeriqid(address, client)
```

**Change B: Update HandleCreateIdentity to pass ageBracket**

The `CreateIdentityRequest` struct needs a new field:

```go
type CreateIdentityRequest struct {
    Keypath    string `json:"keypath"`
    EthKey     string `json:"ethkey"`
    Contract   string `json:"contract,omitempty"`
    RPCURL     string `json:"rpc_url,omitempty"`
    AgeBracket uint8  `json:"age_bracket"` // NEW: 0=Unknown, 1=Under13, 2=Teen, 3=Adult
}
```

And in the handler, find the `AddIDstoIdR` call:
```go
index, err := u2sso.AddIDstoIdR(ethClient, req.EthKey, instance, mpkBytes)
```

Replace with:
```go
index, err := u2sso.AddIDstoIdR(ethClient, req.EthKey, instance, mpkBytes, req.AgeBracket)
```

**Change C: Update HandleRegister**

The `connectToContract` return type is now `*u2sso.Veriqid`, so the register handler automatically gets the right type. No additional changes needed in the register logic since `GetIDSize`, `GetIDIndexfromContract`, and `GetallActiveIDfromContract` all work with `*Veriqid` after Step 6.

**Change D: Update HandleListKeys**

If `HandleListKeys` calls any contract methods, update the instance type accordingly. The `connectToContract` change in (A) handles this implicitly.

### 7.2 Update cmd/client/main.go

The CLI client calls `AddIDstoIdR` in its `create` command. Update the call:

Find:
```go
index, err := u2sso.AddIDstoIdR(ethClient, *ethkey, instance, mpkBytes)
```

Replace with:
```go
// Default to Under13 for CLI testing (age bracket = 1)
index, err := u2sso.AddIDstoIdR(ethClient, *ethkey, instance, mpkBytes, 1)
```

Also update the contract instantiation from `NewU2sso` to `NewVeriqid`:

Find:
```go
instance, err := u2sso.NewU2sso(address, client)
```

Replace with:
```go
instance, err := u2sso.NewVeriqid(address, client)
```

### 7.3 Update cmd/server/main.go

The server uses `NewU2sso` to connect to the contract and calls `GetallActiveIDfromContract`:

Find all occurrences of `NewU2sso` and replace with `NewVeriqid`:

```bash
# Check what needs updating
grep "NewU2sso\|U2sso" cmd/server/main.go
```

Replace each `NewU2sso(...)` with `NewVeriqid(...)`.

### 7.4 Build everything to verify

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

# Build all three binaries
go build ./cmd/client/
go build ./cmd/server/
go build -o bridge-server ./cmd/bridge/
```

**Expected:** All three compile without errors. If any fail:

| Error | Fix |
|-------|-----|
| `undefined: u2sso.U2sso` | You still have a reference to the old type. Change to `u2sso.Veriqid` |
| `too many arguments in call to inst.AddID` | `AddID` now takes 3 parameters (id, id33, ageBracket). Add the age bracket argument |
| `not enough arguments in call to u2sso.AddIDstoIdR` | The updated function now needs `ageBracket uint8` as the last parameter |

### 7.5 Clean up the old binaries

```bash
rm -f client server bridge-server
```

---

## STEP 8: Test the New Contract End-to-End

**Time: ~20 minutes**

### 8.1 Start fresh

You need three terminals. If Ganache was already running, restart it for a clean state:

**Terminal 1 — Ganache:**
```bash
ganache --port 7545
```

Save the first account address and private key (without `0x` prefix).

**Terminal 2 — Deploy and start bridge:**
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle migrate --reset --network development
# Copy the VERIQID contract address (from the 2_deploy_veriqid section)

cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go build -o bridge-server ./cmd/bridge/
./bridge-server -contract 0x<VERIQID_CONTRACT_ADDRESS> -client http://127.0.0.1:7545
```

**Terminal 3 — Testing:**

### 8.2 Test status endpoint

```bash
curl -s http://localhost:9090/api/status | python3 -m json.tool
```

**Expected:**
```json
{
    "status": "ok",
    "version": "0.2.0",
    "contract": "0x...",
    "rpc_url": "http://127.0.0.1:7545"
}
```

### 8.3 Test creating an identity with age bracket

```bash
curl -s -X POST http://localhost:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{
    "keypath": "./test_key1",
    "ethkey": "<GANACHE_PRIVATE_KEY_WITHOUT_0x>",
    "age_bracket": 1
  }' | python3 -m json.tool
```

**Expected:**
```json
{
    "success": true,
    "mpk_hex": "...",
    "index": 0
}
```

The `age_bracket: 1` means "Under 13" — full COPPA protection.

### 8.4 Create a second identity

```bash
curl -s -X POST http://localhost:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{
    "keypath": "./test_key2",
    "ethkey": "<GANACHE_PRIVATE_KEY_WITHOUT_0x>",
    "age_bracket": 2
  }' | python3 -m json.tool
```

**Expected:**
```json
{
    "success": true,
    "mpk_hex": "...",
    "index": 1
}
```

### 8.5 Test the full registration flow

```bash
# Get a challenge
CHALLENGE=$(curl -s http://localhost:9090/api/identity/challenge | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])")
echo "Challenge: $CHALLENGE"

# Generate a registration proof
curl -s -X POST http://localhost:9090/api/identity/register \
  -H "Content-Type: application/json" \
  -d "{
    \"keypath\": \"./test_key1\",
    \"service_name\": \"$(echo -n 'kidstube_test' | sha256sum | cut -d' ' -f1)\",
    \"challenge\": \"$CHALLENGE\"
  }" | python3 -m json.tool
```

**Expected:**
```json
{
    "success": true,
    "proof_hex": "...",
    "spk_hex": "02...",
    "ring_size": 2
}
```

### 8.6 Test auth flow

```bash
CHALLENGE2=$(curl -s http://localhost:9090/api/identity/challenge | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])")

curl -s -X POST http://localhost:9090/api/identity/auth \
  -H "Content-Type: application/json" \
  -d "{
    \"keypath\": \"./test_key1\",
    \"service_name\": \"$(echo -n 'kidstube_test' | sha256sum | cut -d' ' -f1)\",
    \"challenge\": \"$CHALLENGE2\"
  }" | python3 -m json.tool
```

**Expected:**
```json
{
    "success": true,
    "auth_proof_hex": "..."
}
```

The `auth_proof_hex` should be 130 characters (65 bytes hex-encoded).

### 8.7 Test verifier access control via Truffle console

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle console --network development
```

```javascript
let v = await Veriqid.deployed()
let accounts = await web3.eth.getAccounts()

// accounts[0] is deployer = admin + first verifier
// accounts[1] is an unauthorized address

// Try adding an ID from an unauthorized address — should FAIL
try {
  await v.addID(123, 45, 1, {from: accounts[1]})
  console.log("ERROR: Should have reverted!")
} catch(e) {
  console.log("PASS: Unauthorized address blocked:", e.reason || "reverted")
}

// Authorize accounts[1] as a verifier
await v.authorizeVerifier(accounts[1], {from: accounts[0]})

// Now accounts[1] can add an ID
let tx = await v.addID(123, 45, 1, {from: accounts[1]})
console.log("PASS: Authorized verifier added ID, index:", tx.logs[0].args.index.toString())

// accounts[1] (the owner) can revoke their own ID
await v.revokeID(0, {from: accounts[1]})
let state = await v.getState(0)
console.log("PASS: Owner revoked own ID, active:", state)
// Should be: false

.exit
```

### 8.8 Test event emission

In the Truffle console (continue from the same session as 8.7, or re-enter):

```javascript
let v = await Veriqid.deployed()
let accounts = await web3.eth.getAccounts()

// Add a fresh ID and check the event
let tx = await v.addID(999, 88, 2, {from: accounts[0]})
console.log("Event name:", tx.logs[0].event)
// Expected: "IDRegistered"
console.log("Event args:", JSON.stringify(tx.logs[0].args))
// Expected: index, owner, ageBracket
```

Now revoke the ID you just created (use its index from the tx output above, NOT index 0 which was already revoked in Step 8.7):

```javascript
// Get the index of the ID we just added
let newIndex = tx.logs[0].args.index.toString()
console.log("Revoking index:", newIndex)
let tx2 = await v.revokeID(newIndex, {from: accounts[0]})
console.log("Event name:", tx2.logs[0].event)
// Expected: "IDRevoked"

.exit
```

> **Important:** If you try to revoke index 0 here, it will fail with "Veriqid: already revoked" because Step 8.7 already revoked it. Always revoke a fresh, active ID when testing event emission.

---

## STEP 9: Test Batch Retrieval

**Time: ~10 minutes**

### 9.1 Test getBatchIDs in Truffle console

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle console --network development
```

```javascript
let v = await Veriqid.deployed()
let accounts = await web3.eth.getAccounts()

// Add 5 identities — run each line one at a time (do NOT use a for loop
// with await, as Truffle console does not support await inside for loops)
await v.addID(1000, 33, 1, {from: accounts[0]})
await v.addID(1001, 34, 1, {from: accounts[0]})
await v.addID(1002, 35, 1, {from: accounts[0]})
await v.addID(1003, 36, 2, {from: accounts[0]})
await v.addID(1004, 37, 2, {from: accounts[0]})

let size = await v.getIDSize()
console.log("Total IDs:", size.toString())
// Should be 5 on a fresh contract (may be higher if you ran earlier tests)

// Revoke index 2 (the third one)
await v.revokeID(2, {from: accounts[0]})

// Get batch — should return 4 active IDs (skipping the revoked one)
let batch = await v.getBatchIDs(0, 10)
console.log("Active IDs returned:", batch.ids.length)
// Should be 4 on a fresh contract (total minus revoked)
console.log("IDs:", batch.ids.map(x => x.toString()))
// Should show: 1000, 1001, 1003, 1004 (skipping 1002 which was revoked)

// Get active count
let activeCount = await v.getActiveIDCount()
console.log("Active count:", activeCount.toString())
// Should match batch.ids.length

.exit
```

> **Note:** If you ran the Truffle console tests from Steps 8.7–8.8 before this step, your contract already has IDs from those tests. The numbers above assume a **fresh contract** (`truffle migrate --reset`). If the totals are higher than expected, that's normal — the accumulated test data is additive. What matters is that revoking an ID reduces the active count by 1 and `getBatchIDs` excludes it.

---

## STEP 10: Update the Bridge Test Script

**Time: ~10 minutes**

### 10.1 Update test_bridge.sh

If you created `test_bridge.sh` in Phase 2, update the identity creation tests to include the `age_bracket` field:

Find the create identity curl commands and add `"age_bracket": 1`:

```bash
# Old:
# -d '{ "keypath": "./bridge_key1", "ethkey": "'$ETHKEY'" }'

# New:
# -d '{ "keypath": "./bridge_key1", "ethkey": "'$ETHKEY'", "age_bracket": 1 }'
```

### 10.2 Add a new test for verifier access control (optional)

You can add a test that attempts to create an identity from an unauthorized address and verifies it fails. However, this requires a second Ganache account and is more involved. For now, the Truffle console test in Step 8.7 covers this.

---

## STEP 11: Clean Up

**Time: ~5 minutes**

### 11.1 Remove test key files

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
rm -f test_key1 test_key2 client server bridge-server
```

### 11.2 Verify the old U2SSO.go still compiles alongside Veriqid.go

Both files can coexist in `pkg/u2sso/` — they define different types. The old `U2SSO.go` is still used by the existing `cmd/server` if it hasn't been fully migrated. Eventually, once all references to `*U2sso` are replaced with `*Veriqid`, you can remove `U2SSO.go`.

```bash
go build ./pkg/u2sso/
# Should compile with both files present
```

If there's a naming conflict (unlikely since we used `--type Veriqid`), you can rename or delete the old file:

```bash
# Only if there are conflicts:
# mv pkg/u2sso/U2SSO.go pkg/u2sso/U2SSO_legacy.go
```

---

## STEP 12: Phase 3 Completion Checklist

```
[ ] contracts/Veriqid.sol created with all enhancements
[ ] Owner bug FIXED: addID() uses msg.sender (not deployer)
[ ] Verifier registry: authorizedVerifiers mapping + onlyVerifier modifier
[ ] Admin functions: authorizeVerifier(), removeVerifier(), transferAdmin()
[ ] Events: IDRegistered, IDRevoked, VerifierAuthorized, VerifierRemoved, AdminTransferred
[ ] Age bracket: AgeBracket enum + ageBracket field in ID struct
[ ] Batch retrieval: getBatchIDs(start, count) returns active IDs only
[ ] getActiveIDCount() returns count of non-revoked IDs
[ ] getAgeBracket() and getOwner() accessors added
[ ] isVerifier() public check added
[ ] Constructor auto-authorizes deployer as admin + first verifier
[ ] Truffle migration 2_deploy_veriqid.js created
[ ] truffle compile succeeds for Veriqid.sol
[ ] truffle migrate --reset deploys both contracts
[ ] abigen installed (go install github.com/ethereum/go-ethereum/cmd/abigen@latest)
[ ] Veriqid.go Go bindings generated in pkg/u2sso/
[ ] u2ssolib.go updated: all *U2sso → *Veriqid, AddIDstoIdR takes ageBracket
[ ] bridge/bridge.go updated: connectToContract returns *Veriqid, CreateIdentityRequest has age_bracket
[ ] cmd/client/main.go updated: uses NewVeriqid, passes ageBracket to AddIDstoIdR
[ ] cmd/server/main.go updated: uses NewVeriqid
[ ] go build ./cmd/client/ succeeds
[ ] go build ./cmd/server/ succeeds
[ ] go build -o bridge-server ./cmd/bridge/ succeeds
[ ] Bridge starts with Veriqid contract address
[ ] Identity creation via bridge works (with age_bracket)
[ ] Registration proof generation works (ring includes Veriqid IDs)
[ ] Auth proof generation works
[ ] Verifier access control tested (unauthorized address blocked)
[ ] Owner revocation tested (owner can revoke their own ID)
[ ] Admin revocation tested (admin can revoke any ID)
[ ] Event emission tested (IDRegistered, IDRevoked events fire)
[ ] getBatchIDs returns only active IDs (skips revoked)
```

---

## What Each New/Changed File Does (Reference)

### contracts/Veriqid.sol (NEW — ~260 lines)
Enhanced smart contract replacing U2SSO.sol. Contains:
- `AgeBracket` enum — Unknown/Under13/Teen/Adult
- `ID` struct — now includes `ageBracket` field
- `admin` state variable with `onlyAdmin` modifier
- `authorizedVerifiers` mapping with `onlyVerifier` modifier
- `addID(uint256, uint, uint8)` — FIXED: uses `msg.sender` as owner, requires authorized verifier, accepts age bracket
- `revokeID(uint)` — allows owner OR admin to revoke
- `authorizeVerifier(address)` / `removeVerifier(address)` — admin manages verifier list
- `transferAdmin(address)` — admin role transfer
- `getAgeBracket(uint)` / `getOwner(uint)` — new accessors
- `getBatchIDs(uint, uint)` — batch retrieval of active IDs for ring construction
- `getActiveIDCount()` — count of non-revoked IDs
- `isVerifier(address)` — public check
- 5 events: `IDRegistered`, `IDRevoked`, `VerifierAuthorized`, `VerifierRemoved`, `AdminTransferred`

### contracts/migrations/2_deploy_veriqid.js (NEW — 5 lines)
Truffle migration that deploys the Veriqid contract.

### pkg/u2sso/Veriqid.go (NEW — auto-generated)
Go bindings for Veriqid.sol, generated by `abigen`. Provides type-safe Go wrappers:
- `NewVeriqid(address, backend)` — creates contract instance
- `Veriqid.AddID(opts, id, id33, ageBracket)` — registers identity
- `Veriqid.RevokeID(opts, index)` — revokes identity
- `Veriqid.AuthorizeVerifier(opts, verifier)` — authorizes verifier
- `Veriqid.GetBatchIDs(opts, start, count)` — batch read
- `Veriqid.GetActiveIDCount(opts)` — active count
- Event filter/watcher types for all 5 events

### pkg/u2sso/u2ssolib.go (MODIFIED)
Contract interaction functions updated:
- `AddIDstoIdR` — now takes `*Veriqid` + `ageBracket uint8`
- `GetIDfromContract` — takes `*Veriqid`
- `GetIDIndexfromContract` — takes `*Veriqid`
- `GetallActiveIDfromContract` — takes `*Veriqid`

### bridge/bridge.go (MODIFIED)
- `connectToContract` — returns `*u2sso.Veriqid` instead of `*u2sso.U2sso`
- `CreateIdentityRequest` — adds `AgeBracket uint8` field
- `HandleCreateIdentity` — passes `req.AgeBracket` to `AddIDstoIdR`

### cmd/client/main.go (MODIFIED)
- Uses `NewVeriqid` instead of `NewU2sso`
- Passes default age bracket (1 = Under13) to `AddIDstoIdR`

### cmd/server/main.go (MODIFIED)
- Uses `NewVeriqid` instead of `NewU2sso`

---

## How Phase 3 Connects to Later Phases

**Phase 4 (Veriqid Server):** The server demo can use `getAgeBracket()` to show age-appropriate content. The verifier registry enables the "trusted verifier" flow.

**Phase 5 (Browser Extension):** The extension calls the bridge API which now passes `age_bracket` when creating identities. No direct contract interaction needed from the extension.

**Phase 6 (Parent Dashboard):** The dashboard listens for `IDRegistered` and `IDRevoked` events using the event filter types generated in `Veriqid.go`. The `getOwner()` function lets the dashboard show which IDs belong to the connected wallet. The `revokeID()` function (now owner-callable) enables one-click revocation from the dashboard.

**Phase 7 (Demo Platform):** The mock "KidsTube" can call `getAgeBracket()` to enforce age-based content filtering. The `getBatchIDs()` function makes ring construction efficient for the demo.

---

## Next Steps: Phase 4 — Veriqid Server

With Phase 3 complete, the smart contract is production-ready with proper access control, events, and age metadata. Phase 4 forks `cmd/server/main.go` into a Veriqid-branded service demo with configurable service name, proper session management via SQLite for SPK storage, and an age-verification endpoint that platforms can call to check a user's age bracket.
