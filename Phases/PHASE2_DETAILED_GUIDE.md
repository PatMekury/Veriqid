# Veriqid Phase 2: Bridge API - Complete Detailed Guide

**Time estimate: ~4 hours**
**Goal: Create `bridge/bridge.go` — an HTTP JSON API that wraps the CGO crypto library so the browser extension (Phase 5) can call it directly instead of requiring manual CLI copy-paste.**

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
> You will also need to use one of the Ganache private keys (without the `0x` prefix) for any operations that send Ethereum transactions (like creating identities).

---

## BEFORE YOU START: Understanding What You're Building

In Phase 1, the flow was manual:
1. Server generates a challenge → displayed in an HTML form
2. You **copy** the challenge into the CLI client
3. CLI outputs proof values → you **copy** them back into the form
4. Server verifies → success/failure page

This is fine for testing but unusable in production. Phase 2 creates a **Bridge API** — a local HTTP service that sits between the browser extension and the crypto library. The extension sends JSON requests, the bridge calls the same CGO functions as the CLI, and returns JSON responses.

```
Phase 1 (manual):     Browser ←→ Human ←→ CLI ←→ CGO/C
Phase 2 (automated):  Browser Extension ←→ Bridge API ←→ CGO/C
```

**The bridge runs LOCALLY on the user's machine** (like a password manager). The master secret key never leaves the device. The bridge is NOT a cloud service.

### Why Not Just Add JSON Endpoints to the Existing Server?

The existing server (`cmd/server/main.go`) is a **service provider** — it generates challenges and verifies proofs. It represents YouTube, Roblox, etc.

The bridge is a **user-side agent** — it holds the user's secret keys and generates proofs. These are fundamentally different roles:

| | Server (cmd/server) | Bridge (bridge/bridge.go) |
|---|---|---|
| **Runs on** | Service provider's infrastructure | User's local machine |
| **Holds** | Nothing sensitive (public challenges) | Master secret key (msk) |
| **Does** | Verifies proofs | Generates proofs |
| **Talks to** | Browser (HTTP forms) | Browser extension (JSON API) |
| **Trust model** | Untrusted (could be malicious) | Trusted (user's own device) |

---

## PREREQUISITES

Before starting Phase 2, you must have Phase 1 fully complete:

```
[x] libsecp256k1 built and installed with --enable-module-ringcip
[x] go build ./cmd/client/ succeeds (CGO works)
[x] go build ./cmd/server/ succeeds
[x] U2SSO.sol deployed to Ganache (contract address saved)
[x] 2+ identities created (key1, key2 exist)
[x] Ganache running on port 7545
```

You'll also need the contract address and a Ganache private key handy.

---

## STEP 1: Understand the Existing Code You're Wrapping

**Time: ~20 minutes (reading, no coding)**

Before writing the bridge, you need to understand the four operations it will expose. Each maps directly to existing functions in `pkg/u2sso/u2ssolib.go`.

### 1.1 The actual function signatures in u2ssolib.go

These are the **real** signatures you'll be calling from the bridge. Study them carefully:

```go
// Key management
func CreatePasskey(filename string) error
func LoadPasskey(filename string) ([]byte, bool)
func CreateID(mskBytes []byte) []byte

// Challenge generation
func CreateChallenge() []byte

// Proof generation
func RegistrationProof(index int, currentm int, currentN int, serviceName []byte, challenge []byte, mskBytes []byte, idList [][]byte) (string, []byte, bool)
func AuthProof(serviceName []byte, challenge []byte, mskBytes []byte) (string, bool)

// Proof verification (used by the server, not the bridge)
func RegistrationVerify(proofHex string, currentm int, currentN int, serviceName []byte, challenge []byte, idList [][]byte, spkBytes []byte) bool
func AuthVerify(proofHex string, serviceName []byte, challenge []byte, spkBytes []byte) bool

// Contract interaction — NOTE: these take *ethclient.Client and *U2sso instances, not raw strings
func AddIDstoIdR(client *ethclient.Client, sk string, inst *U2sso, id []byte) (int64, error)
func GetIDfromContract(inst *U2sso) (int64, error)
func GetIDIndexfromContract(inst *U2sso, id []byte) (int64, error)
func GetallActiveIDfromContract(inst *U2sso) ([][]byte, error)
```

**Key observations:**
- `LoadPasskey` returns `([]byte, bool)` — NOT `([]byte, error)`. The bool is false if loading fails.
- `CreateID` returns just `[]byte` — no error return (it cannot fail given valid input).
- `CreateChallenge` returns just `[]byte` — no error return.
- `RegistrationProof` returns `(string, []byte, bool)` — the proof as a hex string, the spk as bytes, and a success bool.
- `AuthProof` returns `(string, bool)` — the proof as a hex string and a success bool. It does NOT return the spk separately.
- The contract functions (`AddIDstoIdR`, `GetIDIndexfromContract`, `GetallActiveIDfromContract`) take **instances** (`*ethclient.Client`, `*U2sso`), not raw URL/address strings. This means the bridge needs a helper to create these instances from strings.

### 1.2 Operation: Create Identity

**CLI equivalent:**
```bash
go run ./cmd/client -contract 0x... -ethkey <key> -command create -keypath ./key1
```

**What it does:**
1. Calls `CreatePasskey(keypath)` → generates 32 random bytes via OpenSSL `RAND_bytes()`, saves to file
2. Calls `LoadPasskey(keypath)` → reads back the 32-byte msk (returns `[]byte, bool`)
3. Calls `CreateID(mskBytes)` → derives 33-byte compressed mpk (returns `[]byte`)
4. Calls `AddIDstoIdR(ethClient, ethkey, instance, mpkBytes)` → sends Ethereum transaction to register mpk on-chain (returns `int64, error`)

**Bridge API equivalent:** `POST /api/identity/create`

### 1.3 Operation: Generate Registration Proof

**CLI equivalent:**
```bash
go run ./cmd/client -contract 0x... -command register -keypath ./key1 -sname <hex> -challenge <hex>
```

**What it does:**
1. Loads msk from keypath
2. Derives mpk via `CreateID(msk)`
3. Finds our index via `GetIDIndexfromContract(instance, mpk)` (returns `int64, error`)
4. Gets ID count via `instance.GetIDSize(nil)` and calculates ring parameters
5. Gets all active IDs via `GetallActiveIDfromContract(instance)` (returns `[][]byte, error`)
6. Calls `RegistrationProof(index, currentm, currentN, serviceName, challenge, mskBytes, idList)` → returns `(string, []byte, bool)`

**Bridge API equivalent:** `POST /api/identity/register`

### 1.4 Operation: Generate Auth Proof

**CLI equivalent:**
```bash
go run ./cmd/client -contract 0x... -command auth -keypath ./key1 -sname <hex> -challenge <hex>
```

**What it does:**
1. Loads msk from keypath
2. Calls `AuthProof(serviceName, challenge, mskBytes)` → returns `(string, bool)` — the hex-encoded 65-byte proof

**Bridge API equivalent:** `POST /api/identity/auth`

### 1.5 Operation: Generate Challenge

1. Calls `CreateChallenge()` → 32 random bytes (returns `[]byte`)

**Bridge API equivalent:** `GET /api/identity/challenge`

---

## STEP 2: Create the Bridge File Structure

**Time: ~5 minutes**

### 2.1 Navigate to the Veriqid directory

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
```

### 2.2 Create the directories

```bash
mkdir -p cmd/bridge
```

Your directory structure will be:
```
Veriqid/
├── bridge/
│   └── bridge.go          ← The bridge HTTP handlers (library)
├── cmd/
│   ├── bridge/
│   │   └── main.go        ← Bridge entry point (main binary)
│   ├── client/
│   │   └── main.go        ← CLI tool (existing)
│   └── server/
│       └── main.go        ← Service provider server (existing)
├── pkg/u2sso/
│   ├── u2ssolib.go        ← CGO crypto wrapper (existing)
│   └── U2SSO.go           ← Contract bindings (existing)
└── ...
```

**Why separate `bridge/` and `cmd/bridge/`?**

- `bridge/bridge.go` is a **Go package** (importable library) containing the HTTP handler functions, request/response types, and business logic.
- `cmd/bridge/main.go` is the **executable entry point** — it parses CLI flags, creates a bridge instance, and starts the HTTP server.

---

## STEP 3: Define the Request/Response Types

**Time: ~15 minutes**

### 3.1 Understanding the JSON API contract

The bridge communicates via JSON. Each endpoint has a well-defined request and response structure:

**Create Identity:**
```
Request:  { "keypath": "./key1", "ethkey": "abc123...", "contract": "0x...", "rpc_url": "http://..." }
Response: { "success": true, "mpk_hex": "02abc...", "index": 0 }
```

**Register (generate proof for signup):**
```
Request:  { "keypath": "./key1", "service_name": "a1b2c3...", "challenge": "d4e5f6...", "contract": "0x...", "rpc_url": "http://..." }
Response: { "success": true, "proof_hex": "...", "spk_hex": "02abc...", "ring_size": 2 }
```

**Authenticate (generate proof for login):**
```
Request:  { "keypath": "./key1", "service_name": "a1b2c3...", "challenge": "d4e5f6..." }
Response: { "success": true, "auth_proof_hex": "..." }
```

**Challenge:**
```
Request:  GET (no body)
Response: { "challenge": "a1b2c3d4e5f6..." }
```

### 3.2 Important type details

- `Index` and `RingSize` are `int64` (not `int`) because they come from `*big.Int` contract calls
- `AuthResponse` does NOT include `spk_hex` — `AuthProof()` returns only the proof hex, not the spk separately
- All hex encoding uses `hex.EncodeToString()` (from `encoding/hex`), not `fmt.Sprintf("%x", ...)`

---

## STEP 4: Write the Bridge Core Logic

**Time: ~45 minutes**

### 4.1 The Bridge struct and helpers

The bridge needs to hold configuration that persists across requests and a helper to connect to the Ethereum contract:

```go
type Bridge struct {
    DefaultContract string
    DefaultRPCURL   string
}

func NewBridge(contract, rpcURL string) *Bridge {
    if rpcURL == "" {
        rpcURL = "http://127.0.0.1:7545"
    }
    return &Bridge{DefaultContract: contract, DefaultRPCURL: rpcURL}
}
```

### 4.2 The `connectToContract` helper — CRITICAL

The u2ssolib.go contract functions take `*ethclient.Client` and `*U2sso` instances, NOT raw strings. The bridge needs a helper that:
1. Dials the Ethereum client
2. Verifies the contract exists at the address
3. Creates the contract instance

```go
func connectToContract(rpcURL, contractAddr string) (*ethclient.Client, *u2sso.U2sso, error) {
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

    instance, err := u2sso.NewU2sso(address, client)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to instantiate U2SSO contract: %w", err)
    }

    return client, instance, nil
}
```

**This helper is called by HandleCreateIdentity, HandleRegister, and HandleListKeys** — any handler that touches the blockchain.

### 4.3 Handler: POST /api/identity/create

```go
func (b *Bridge) HandleCreateIdentity(w http.ResponseWriter, r *http.Request) {
    // ... validate method, decode JSON, check required fields ...

    // Connect to Ethereum and the contract
    ethClient, instance, err := connectToContract(rpcURL, contract)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }

    // Step 1: CreatePasskey returns error (not log.Fatal)
    if err := u2sso.CreatePasskey(req.Keypath); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create passkey: "+err.Error())
        return
    }

    // Step 2: LoadPasskey returns ([]byte, bool) — NOT ([]byte, error)
    mskBytes, ok := u2sso.LoadPasskey(req.Keypath)
    if !ok {
        writeError(w, http.StatusInternalServerError, "failed to load passkey after creation")
        return
    }

    // Step 3: CreateID returns []byte — no error
    mpkBytes := u2sso.CreateID(mskBytes)

    // Step 4: AddIDstoIdR takes (ethClient, ethkey, instance, mpkBytes)
    index, err := u2sso.AddIDstoIdR(ethClient, req.EthKey, instance, mpkBytes)
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
```

**Note:** `AddIDstoIdR` takes the `*ethclient.Client` as first argument, the private key string as second, the `*U2sso` instance as third, and the mpk bytes as fourth. The old guide incorrectly showed it taking raw strings.

### 4.4 Handler: POST /api/identity/register

This is the most complex handler because it needs to:
1. Connect to the contract
2. Decode hex inputs (service_name and challenge come as hex strings)
3. Calculate ring parameters (currentm from idsize)
4. Find our index in the ring
5. Fetch all active IDs
6. Generate the proof

```go
func (b *Bridge) HandleRegister(w http.ResponseWriter, r *http.Request) {
    // ... validate, decode JSON ...

    // Decode hex inputs to bytes
    serviceName, err := hex.DecodeString(req.ServiceName)
    challenge, err := hex.DecodeString(req.Challenge)

    // Connect to contract
    _, instance, err := connectToContract(rpcURL, contract)

    // Load master secret key — returns ([]byte, bool)
    mskBytes, ok := u2sso.LoadPasskey(req.Keypath)
    if !ok { ... }

    // Get total ID count and calculate ring parameters
    idsize, err := instance.GetIDSize(nil)
    currentm := 1
    ringSize := 1
    for i := 1; i < u2sso.M; i++ {
        ringSize = u2sso.N * ringSize
        if ringSize >= int(idsize.Int64()) {
            currentm = i
            break
        }
    }

    // Find our index — GetIDIndexfromContract takes (instance, mpkBytes)
    mpkBytes := u2sso.CreateID(mskBytes)
    index, err := u2sso.GetIDIndexfromContract(instance, mpkBytes)

    // Fetch all active IDs — GetallActiveIDfromContract takes (instance)
    idList, err := u2sso.GetallActiveIDfromContract(instance)

    // Generate proof — RegistrationProof returns (string, []byte, bool)
    proofHex, spkBytes, ok := u2sso.RegistrationProof(
        int(index), currentm, int(idsize.Int64()),
        serviceName, challenge, mskBytes, idList,
    )
    if !ok { ... }

    writeJSON(w, RegisterResponse{
        Success:  true,
        ProofHex: proofHex,
        SpkHex:   hex.EncodeToString(spkBytes),
        RingSize: idsize.Int64(),
    })
}
```

**Key difference from the old guide:** The old guide showed `GetallActiveIDfromContract(contract, rpcURL)` taking strings. The real function takes `GetallActiveIDfromContract(instance)` — an `*U2sso` instance. Similarly, ring parameter calculation must happen in the bridge, not in u2ssolib.

### 4.5 Handler: POST /api/identity/auth

```go
func (b *Bridge) HandleAuth(w http.ResponseWriter, r *http.Request) {
    // ... validate, decode JSON, decode hex inputs ...

    // Load master secret key — returns ([]byte, bool)
    mskBytes, ok := u2sso.LoadPasskey(req.Keypath)
    if !ok { ... }

    // AuthProof returns (string, bool) — NOT (bytes, bytes, error)
    // The string is the hex-encoded 65-byte proof
    // It does NOT return the spk separately
    authProofHex, ok := u2sso.AuthProof(serviceName, challenge, mskBytes)
    if !ok { ... }

    writeJSON(w, AuthResponse{
        Success:      true,
        AuthProofHex: authProofHex,
    })
}
```

**Important:** `AuthProof` returns `(string, bool)`, not `([]byte, []byte, error)` as the old guide showed. The spk is not returned separately — it can be re-derived from the same msk + service_name if needed.

### 4.6 Handler: GET /api/identity/challenge

```go
func (b *Bridge) HandleChallenge(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeError(w, http.StatusMethodNotAllowed, "method not allowed, use GET")
        return
    }

    // CreateChallenge returns []byte — no error
    challengeBytes := u2sso.CreateChallenge()

    writeJSON(w, ChallengeResponse{
        Challenge: hex.EncodeToString(challengeBytes),
    })
}
```

---

## STEP 5: Add CORS Middleware

**Time: ~10 minutes**

### 5.1 Why CORS is required

The browser extension makes requests from `chrome-extension://...` origin to `http://localhost:9090`. Without CORS headers, the browser blocks these requests.

### 5.2 CORS middleware implementation

```go
func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        w.Header().Set("Access-Control-Max-Age", "86400")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Security note:** Using `Access-Control-Allow-Origin: *` is acceptable because the bridge runs on `localhost` only and the extension origin changes per Chrome install.

---

## STEP 6: Add Health Check, Status, and List Endpoints

**Time: ~15 minutes**

### 6.1 GET /api/status

```go
func (b *Bridge) HandleStatus(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, StatusResponse{
        Status:   "ok",
        Version:  "0.2.0",
        Contract: b.DefaultContract,
        RPCURL:   b.DefaultRPCURL,
    })
}
```

### 6.2 POST /api/identity/list

This handler scans a directory for 32-byte key files and optionally checks on-chain status using the contract instance directly (via `GetIDIndex` and `GetState` on the instance, since those are contract methods).

---

## STEP 7: CGO Import Block — REQUIRED in Both Files

**Time: ~5 minutes**

Both `bridge/bridge.go` AND `cmd/bridge/main.go` need a CGO import block. This is because the bridge links against the C crypto library. Without it, you'll get linker errors.

**In `bridge/bridge.go`:**
```go
// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lcrypto -lsecp256k1
// #include <stdlib.h>
// #include <stdint.h>
// #include <string.h>
// #include <openssl/rand.h>
// #include <secp256k1.h>
// #include <secp256k1_ringcip.h>
import "C"
```

**In `cmd/bridge/main.go`:**
```go
// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lcrypto -lsecp256k1
// #include <stdlib.h>
// #include <stdint.h>
// #include <string.h>
// #include <openssl/rand.h>
// #include <secp256k1.h>
// #include <secp256k1_ringcip.h>
import "C"
```

**These comment directives MUST be directly above `import "C"` with NO blank lines in between.** This is a CGO requirement.

---

## STEP 8: Wire Up the Router

**Time: ~5 minutes**

```go
func (b *Bridge) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("/api/status", b.HandleStatus)
    mux.HandleFunc("/api/identity/create", b.HandleCreateIdentity)
    mux.HandleFunc("/api/identity/register", b.HandleRegister)
    mux.HandleFunc("/api/identity/auth", b.HandleAuth)
    mux.HandleFunc("/api/identity/list", b.HandleListKeys)
    mux.HandleFunc("/api/identity/challenge", b.HandleChallenge)
}
```

---

## STEP 9: Write the Complete `bridge/bridge.go`

**Time: ~20 minutes**

Assemble everything into the final file. The complete file includes:
- CGO import block
- All imports (`context`, `encoding/hex`, `encoding/json`, `fmt`, `math/big`, `net/http`, `os`, `path/filepath`, plus `go-ethereum` packages)
- `Bridge` struct and `NewBridge()`
- `connectToContract()` helper
- All request/response types (with `int64` for Index and RingSize)
- `writeError()` and `writeJSON()` helpers
- `CORSMiddleware()`
- All 6 handlers
- `RegisterRoutes()`

The actual implementation file is `bridge/bridge.go` in the repository (667 lines).

---

## STEP 10: Write the Bridge Entry Point (`cmd/bridge/main.go`)

**Time: ~15 minutes**

```go
package main

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lcrypto -lsecp256k1
// #include <stdlib.h>
// #include <stdint.h>
// #include <string.h>
// #include <openssl/rand.h>
// #include <secp256k1.h>
// #include <secp256k1_ringcip.h>
import "C"

import (
    "flag"
    "fmt"
    "log"
    "net/http"

    "github.com/patmekury/veriqid/bridge"
)

func main() {
    contractAddr := flag.String("contract", "", "U2SSO smart contract address (required)")
    clientAddr := flag.String("client", "http://127.0.0.1:7545", "Ethereum JSON-RPC endpoint")
    port := flag.Int("port", 9090, "Port for the bridge API to listen on")
    flag.Parse()

    if *contractAddr == "" {
        log.Fatal("Error: -contract flag is required.")
    }

    b := bridge.NewBridge(*contractAddr, *clientAddr)
    mux := http.NewServeMux()
    b.RegisterRoutes(mux)
    handler := bridge.CORSMiddleware(mux)

    addr := fmt.Sprintf("127.0.0.1:%d", *port)
    fmt.Println("===========================================")
    fmt.Println("  Veriqid Bridge API")
    fmt.Println("===========================================")
    fmt.Printf("  Listening:  http://%s\n", addr)
    fmt.Printf("  Contract:   %s\n", *contractAddr)
    fmt.Printf("  RPC:        %s\n", *clientAddr)
    fmt.Println("-------------------------------------------")
    fmt.Println("  Endpoints:")
    fmt.Println("    GET  /api/status")
    fmt.Println("    POST /api/identity/create")
    fmt.Println("    POST /api/identity/register")
    fmt.Println("    POST /api/identity/auth")
    fmt.Println("    POST /api/identity/list")
    fmt.Println("    GET  /api/identity/challenge")
    fmt.Println("===========================================")

    // IMPORTANT: 127.0.0.1 only — the bridge holds secret keys
    log.Fatal(http.ListenAndServe(addr, handler))
}
```

**CRITICAL: `127.0.0.1` not `0.0.0.0`** — the bridge holds secret keys and must not be accessible from the network.

---

## STEP 11: Build and Test the Bridge

**Time: ~20 minutes**

### 11.1 Build the binary

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

# IMPORTANT: Use -o to name the output binary, because go build would try to name
# it "bridge" which conflicts with the existing bridge/ directory
go build -o bridge-server ./cmd/bridge/
```

**If it fails with `build output "bridge" already exists and is a directory`:** You forgot the `-o bridge-server` flag. Go tries to name the binary `bridge` which conflicts with the `bridge/` package directory.

### 11.2 Start the prerequisites

You need three terminals running:

**Terminal 1 — Ganache:**
```bash
ganache --port 7545
```

**Terminal 2 — Deploy contract and start bridge:**
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle migrate --reset --network development
# Copy the contract address from the output

cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
./bridge-server -contract 0x<YOUR_CONTRACT_ADDRESS> -client http://127.0.0.1:7545
```

You should see the startup banner:
```
===========================================
  Veriqid Bridge API
===========================================
  Listening:  http://127.0.0.1:9090
  Contract:   0x37e763BAf75360...
  RPC:        http://127.0.0.1:7545
-------------------------------------------
```

**Terminal 3 — Testing:**

### 11.3 Test the status endpoint

```bash
curl -s http://localhost:9090/api/status | python3 -m json.tool
```

**Expected output:**
```json
{
    "status": "ok",
    "version": "0.2.0",
    "contract": "0x37e763BAf75360Ade074D979d7059f230A8282F7",
    "rpc_url": "http://127.0.0.1:7545"
}
```

### 11.4 Test creating an identity

```bash
curl -s -X POST http://localhost:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{
    "keypath": "./bridge_key1",
    "ethkey": "<GANACHE_PRIVATE_KEY_WITHOUT_0x>"
  }' | python3 -m json.tool
```

**Expected output:**
```json
{
    "success": true,
    "mpk_hex": "09e574e4404a67070918...",
    "index": 0
}
```

### 11.5 Test the full flow (challenge → register → auth)

See the test script in Step 12 for automated testing.

---

## STEP 12: Test Script

**Time: ~15 minutes**

### 12.1 Create the test script

Create `test_bridge.sh` in the Veriqid directory. This script accepts a Ganache private key as a CLI argument (without the `0x` prefix):

```bash
#!/bin/bash
# Usage: ./test_bridge.sh <GANACHE_PRIVATE_KEY_WITHOUT_0x>
#
# End-to-end test for the Veriqid Bridge API.
# Requires: bridge running on port 9090, Ganache running on 7545
```

The test script covers 7 test groups with 17 tests:
1. **Status** — `GET /api/status` returns ok
2. **Challenge** — `GET /api/identity/challenge` returns 64-char hex
3. **Create** — creates 2 identities, verifies key files are 32 bytes
4. **Register** — generates a registration proof with valid proof_hex, spk_hex, ring_size
5. **Auth** — generates a 130-char (65-byte) auth proof
6. **Error handling** — tests wrong method, missing fields, invalid JSON, nonexistent key
7. **CORS** — verifies CORS headers on OPTIONS preflight

### 12.2 Run the test

```bash
# If on WSL, fix line endings first:
dos2unix test_bridge.sh
chmod +x test_bridge.sh

./test_bridge.sh <GANACHE_PRIVATE_KEY_WITHOUT_0x>
```

**Expected output:**
```
=== Test 1: GET /api/status ===
  PASS Status endpoint (status=ok)
  PASS Version present (version=0.2.0)
...
=== Test 6: Error handling ===
  PASS GET on POST endpoint (success=False)
  PASS Missing ethkey (success=False)
  PASS Invalid JSON (success=False)
  PASS Nonexistent key (success=False)
...
===========================================
  ALL 17 TESTS PASSED
===========================================
```

### 12.3 Note about dos2unix on WSL

If you see `cannot execute: required file not found` when running the script, it's a Windows CRLF line ending issue (same issue from Phase 1 with the C build scripts). Fix with:

```bash
dos2unix test_bridge.sh
```

---

## STEP 13: Troubleshooting

### 13.1 Common errors

| Error | Cause | Fix |
|-------|-------|-----|
| `build output "bridge" already exists and is a directory` | `go build ./cmd/bridge/` tries to name binary `bridge` | Use `go build -o bridge-server ./cmd/bridge/` |
| `no contract found at address 0x...` | Ganache was restarted (it's ephemeral) | Redeploy with `truffle migrate --reset --network development`, restart bridge with new address |
| `bridge/bridge.go: undefined: u2sso.CreatePasskey` | Import path mismatch | Verify `go.mod` module path matches import |
| `connection refused` on curl | Bridge isn't running | Start with `./bridge-server -contract 0x...` |
| `failed to load passkey` | Key file doesn't exist at that path, or wrong working directory | Key files are created relative to the bridge process working directory |
| `cannot execute: required file not found` | Windows CRLF line endings in shell scripts | Run `dos2unix <script>` first |
| Bridge crashes on bad input | `log.Fatal` in u2ssolib.go kills the process | See fix below in Section 13.2 |

### 13.2 CRITICAL FIX: Graceful error handling in u2ssolib.go

The original `u2ssolib.go` used `log.Fatal()` for error handling. This kills the **entire process**, meaning any bad request (like a nonexistent key file) would crash the bridge server.

**The fix** (already applied): All `log.Fatal` calls in `u2ssolib.go` were replaced with graceful error returns:

- `CreatePasskey(filename string)` — now returns `error` instead of calling `log.Fatal`
- `AddIDstoIdR(...)` — now returns `(int64, error)` instead of `int64` + `log.Fatal`
- `GetIDfromContract(inst)` — now returns `(int64, error)`
- `GetIDIndexfromContract(inst, id)` — now returns `(int64, error)`
- `GetallActiveIDfromContract(inst)` — now returns `([][]byte, error)`
- `LoadPasskey` — returns `nil, false` on error instead of `log.Fatal`
- `RegistrationVerify` and `AuthVerify` — return `false` on hex decode errors instead of `log.Fatal`

**All callers were also updated:**
- `cmd/client/main.go` — handles new error returns
- `bridge/bridge.go` — handles new error returns
- `cmd/server/main.go` — handles `GetallActiveIDfromContract` returning `([][]byte, error)`

This is essential for any long-running HTTP server — never use `log.Fatal` in handler code.

### 13.3 Debugging CGO issues

If the bridge compiles but crashes at runtime with a segfault:

```bash
GODEBUG=cgocheck=2 go run ./cmd/bridge -contract 0x...
```

---

## STEP 14: Phase 2 Completion Checklist

```
[ ] bridge/bridge.go contains all handlers (create, register, auth, challenge, list, status)
[ ] bridge/bridge.go includes CGO import block and connectToContract helper
[ ] cmd/bridge/main.go has CLI flags, CGO import block, and startup logic
[ ] go build -o bridge-server ./cmd/bridge/ compiles without errors
[ ] Bridge starts and shows the endpoint banner
[ ] GET /api/status returns {"status": "ok", ...}
[ ] GET /api/identity/challenge returns 64-char hex challenge
[ ] POST /api/identity/create creates a key file and registers on-chain
[ ] POST /api/identity/register generates proof_hex, spk_hex, ring_size
[ ] POST /api/identity/auth generates auth_proof_hex (130 chars)
[ ] CORS headers present on all responses (Access-Control-Allow-Origin: *)
[ ] OPTIONS preflight returns 204 with CORS headers
[ ] Error responses return proper JSON with success=false (no crashes)
[ ] Bridge binds to 127.0.0.1 (not 0.0.0.0)
[ ] Test script passes all 17 tests
[ ] u2ssolib.go uses graceful error returns (no log.Fatal in library code)
```

---

## What Each New File Does (Reference)

### bridge/bridge.go (667 lines)
The bridge HTTP handlers (importable package). Contains:
- CGO import block (required for C library linking)
- `Bridge` struct — holds default contract and RPC URL configuration
- `connectToContract()` — dials ethclient, verifies contract exists, creates instance
- `NewBridge()` — constructor with default resolution
- `HandleCreateIdentity()` — wraps CreatePasskey + LoadPasskey + CreateID + AddIDstoIdR
- `HandleRegister()` — wraps LoadPasskey + GetIDSize + ring param calculation + GetIDIndexfromContract + GetallActiveIDfromContract + RegistrationProof
- `HandleAuth()` — wraps LoadPasskey + AuthProof
- `HandleChallenge()` — wraps CreateChallenge
- `HandleListKeys()` — scans directory for 32-byte key files, checks on-chain status via instance
- `HandleStatus()` — returns bridge health + configuration
- `CORSMiddleware()` — adds CORS headers for browser extension access
- `RegisterRoutes()` — wires all handlers to a ServeMux
- All request/response JSON types (with `int64` for Index/RingSize)
- `writeError()` and `writeJSON()` helpers

### cmd/bridge/main.go (78 lines)
The bridge entry point (executable). Includes CGO import block, parses CLI flags (`-contract`, `-client`, `-port`), creates a `Bridge` instance, wraps routes in CORS middleware, starts HTTP server on `127.0.0.1:9090`.

---

## How the Bridge Connects to Phase 5 (Browser Extension)

```
1. Extension detects a "Sign up with Veriqid" button on KidsTube.com
2. Extension extracts the challenge and service_name from the page
3. Extension calls POST http://localhost:9090/api/identity/register
   with { keypath, service_name, challenge }
4. Bridge generates proof via CGO → returns { proof_hex, spk_hex, ring_size }
5. Extension auto-fills the signup form with the proof values
6. Extension submits the form
7. Service verifies the proof server-side → registration complete
```

For login:
```
1. Extension detects a "Log in with Veriqid" button
2. Extension extracts challenge and service_name
3. Extension calls POST http://localhost:9090/api/identity/auth
4. Bridge returns { auth_proof_hex }
5. Extension submits to the service
6. Service verifies → login complete
```

---

## Next Steps: Phase 3 — Enhanced Smart Contract

With Phase 2 complete, you have a programmatic API for all crypto operations. Phase 3 creates `contracts/Veriqid.sol` which fixes the owner bug in `U2SSO.sol` (so parents can actually revoke identities), adds a verifier registry (only authorized entities like pediatricians can register children), and adds events for the parent dashboard (Phase 6) to listen to.
