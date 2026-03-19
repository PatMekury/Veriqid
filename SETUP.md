# Veriqid — Setup & Testing Guide

Complete instructions for cloning the repository, building the cryptographic library, deploying smart contracts, and running the full Veriqid system locally.

**Estimated total time: ~45 minutes** (on a fresh Ubuntu/WSL2 environment)

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Git | Any | Clone the repository |
| Go | 1.22+ | Server, client, bridge, CGO bindings |
| Node.js | 18+ | Hardhat/Truffle, Ganache (local blockchain) |
| Ganache CLI | Latest | Local Ethereum blockchain for testing |
| Truffle | Latest | Smart contract compilation & deployment |
| OpenSSL dev headers | libssl-dev | Required by the C crypto library via CGO |
| Build tools | gcc, autoconf, automake, libtool | Compiling libsecp256k1 Boquila fork |
| WSL2 or Linux | Ubuntu 22.04+ | **Windows users must use WSL2** — the C library uses Unix autotools |

---

## Step 1: Clone the Repository (~2 min)

```bash
git clone https://github.com/nicola-2010/U2SSO.git
cd U2SSO/Veriqid
```

The `Veriqid/` directory is the main project. The `crypto-dbpoe/` directory (one level up in `U2SSO/`) contains the custom libsecp256k1 fork that must be copied in.

```bash
# Copy the crypto library into the Veriqid project
cp -r ../crypto-dbpoe ./crypto-dbpoe
```

Verify the copy worked:

```bash
ls crypto-dbpoe/include/secp256k1_ringcip.h
# Should show the file path — this is the ring signature header
```

---

## Step 2: Install System Dependencies (~10 min)

### On Ubuntu / WSL2 (recommended)

```bash
# Update packages
sudo apt update && sudo apt upgrade -y

# Install Go 1.22+
sudo rm -rf /usr/local/go
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
rm go1.22.5.linux-amd64.tar.gz

# Install C build toolchain + OpenSSL dev headers
sudo apt install -y build-essential autoconf automake libtool libssl-dev pkg-config dos2unix

# Install Node.js 18+
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt install -y nodejs

# Install Ganache and Truffle globally
sudo npm install -g ganache truffle
```

### On macOS

```bash
brew install go node autoconf automake libtool openssl pkg-config
npm install -g ganache truffle
```

### Verify everything

```bash
go version          # go1.22.x or higher
go env CGO_ENABLED  # Must show: 1
node --version      # v18.x or higher
ganache --version   # Any version
truffle version     # Truffle v5.x + Solidity 0.8.x
gcc --version       # Any version
```

---

## Step 3: Build libsecp256k1 with Boquila Extensions (~10 min)

This is the **most critical step**. The library provides ring signatures and Boquila key derivation — the core cryptographic engine.

```bash
cd crypto-dbpoe

# Fix Windows line endings if cloned on Windows
dos2unix $(find . -type f \( -name "*.sh" -o -name "*.ac" -o -name "*.am" -o -name "*.m4" -o -name "Makefile*" -o -name "*.in" \) 2>/dev/null) 2>/dev/null || true

# Fix two known build bugs in the crypto-dbpoe fork
# Bug A: libtoolize macro conflict (singular → plural)
sed -i 's/AC_CONFIG_MACRO_DIR(\[build-aux\/m4\])/AC_CONFIG_MACRO_DIRS([build-aux\/m4])/' configure.ac
sed -i '/^ACLOCAL_AMFLAGS/d' Makefile.am

# Bug B: Missing --enable-module-ringcip flag definition
sed -i '/\[enable_module_aggsig=no\])/a\
\
AC_ARG_ENABLE(module_ringcip,\
    AS_HELP_STRING([--enable-module-ringcip],[enable ring signature CIP module]),\
    [enable_module_ringcip=$enableval],\
    [enable_module_ringcip=no])' configure.ac

# Generate build scripts
./autogen.sh

# Configure with ring signature module enabled
./configure --enable-module-ringcip --enable-experimental

# VERIFY the ringcip module is actually enabled
grep "ENABLE_MODULE_RINGCIP" src/libsecp256k1-config.h
# MUST show: #define ENABLE_MODULE_RINGCIP 1
# If it shows #undef, the bug fixes above didn't apply — see Troubleshooting below

# Compile
make

# Install system-wide
sudo make install
sudo ldconfig
```

Verify the installation:

```bash
ls /usr/local/lib/libsecp256k1*         # .so, .a files should exist
ls /usr/local/include/secp256k1_ringcip.h  # Header must exist
ldconfig -p | grep secp256k1            # Linker must find it
```

Return to the project root:

```bash
cd ..
```

---

## Step 4: Verify Go + CGO Compilation (~2 min)

```bash
# Download Go dependencies
go mod tidy

# Build all Go binaries — this is the key test
go build ./cmd/client/
go build ./cmd/server/
go build -o bridge-server ./cmd/bridge/
go build -o veriqid-server ./cmd/veriqid-server/

# If all four succeed without errors, your toolchain is working
```

Clean up test binaries:

```bash
rm -f client server bridge-server veriqid-server
```

### Common CGO errors

| Error | Fix |
|-------|-----|
| `secp256k1.h: No such file or directory` | Run `sudo make install` in `crypto-dbpoe/` |
| `secp256k1_ringcip.h: No such file or directory` | Same — ensure `secp256k1_ringcip.h` is in `/usr/local/include/` |
| `cannot find -lsecp256k1` | Run `sudo ldconfig` after `make install` |
| `undefined reference to secp256k1_ringcip_*` | Rebuild with `--enable-module-ringcip --enable-experimental` |
| `openssl/rand.h: No such file or directory` | `sudo apt install libssl-dev` |

---

## Step 5: Deploy Smart Contracts (~5 min)

### 5.1 Start Ganache (Terminal 1)

Open a **separate terminal** — Ganache stays running in the foreground:

```bash
ganache --port 7545
```

You will see 10 test accounts and their private keys. **Save these two values:**

1. **First account address** — e.g., `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266`
2. **First private key** — e.g., `ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80` (without the `0x` prefix!)

### 5.2 Deploy the contract (Terminal 2)

```bash
cd contracts

# Set up npm if first time
npm init -y 2>/dev/null
npm install 2>/dev/null

# Copy Solidity files into the Truffle contracts/ subfolder
mkdir -p contracts
cp U2SSO.sol contracts/
cp Veriqid.sol contracts/
cp contracts/VeriqidV2.sol contracts/ 2>/dev/null

# Deploy
truffle migrate --network development
```

**Save the contract address** from the output — you need it for every subsequent command.

Return to project root:

```bash
cd ..
```

---

## Step 6: Create Test Identities (~3 min)

Ring signatures require at least 2 identities in the anonymity set.

Replace `<CONTRACT>` with your contract address and `<ETHKEY>` with the Ganache private key (without `0x`):

```bash
# Create identity 1
go run ./cmd/client \
  -contract <CONTRACT> \
  -ethkey <ETHKEY> \
  -command create \
  -keypath ./key1

# Create identity 2
go run ./cmd/client \
  -contract <CONTRACT> \
  -ethkey <ETHKEY> \
  -command create \
  -keypath ./key2
```

Expected output for each:

```
Found the contract at 0x...
Passkey created successfully
Added ID to index: 0   (then 1 for the second)
```

---

## Step 7: Run the Full System (~5 min)

You need **3–4 terminals** running simultaneously:

### Terminal 1 — Ganache (already running from Step 5)

```bash
ganache --port 7545
```

### Terminal 2 — Bridge API (local proof generator)

```bash
go build -o bridge-server ./cmd/bridge/
./bridge-server -contract <CONTRACT> -client http://127.0.0.1:7545
```

Expected output:

```
===========================================
  Veriqid Bridge API
===========================================
  Listening:  http://127.0.0.1:9090
  Contract:   0x...
  RPC:        http://127.0.0.1:7545
===========================================
```

### Terminal 3 — Veriqid Server (KidsTube demo)

```bash
go run ./cmd/veriqid-server \
  -contract <CONTRACT> \
  -ethkey <ETHKEY> \
  -service "KidsTube" \
  -port 8080 \
  -master-secret "test-secret-change-in-production"
```

Expected output:

```
===========================================
  KidsTube — Powered by Veriqid + VE-ASC
===========================================
  Server:     http://localhost:8080
  Service:    KidsTube
  VE-ASC:
    Protocol:     VE-ASC v1.0
    Merkle depth: 20 (max 1048576 identities)
===========================================
```

### Open in browser

Navigate to **http://localhost:8080** — you should see the KidsTube landing page with a "Sign up with Veriqid" button.

---

## Step 8: Test the Solution

### 8.1 Quick API smoke test

```bash
# Bridge health check
curl -s http://localhost:9090/api/status | python3 -m json.tool

# Generate a challenge
curl -s http://localhost:9090/api/identity/challenge | python3 -m json.tool

# Create an identity via the bridge
curl -s -X POST http://localhost:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{
    "keypath": "./bridge_test_key",
    "ethkey": "<ETHKEY>"
  }' | python3 -m json.tool
```

### 8.2 Run the automated bridge test suite

```bash
# Fix line endings if on Windows/WSL
dos2unix test_bridge.sh 2>/dev/null
chmod +x test_bridge.sh

# Run all 17 tests
./test_bridge.sh <ETHKEY>
```

Expected:

```
=== Test 1: GET /api/status ===
  PASS Status endpoint (status=ok)
...
===========================================
  ALL 17 TESTS PASSED
===========================================
```

### 8.3 Run the Veriqid server test suite

```bash
dos2unix test_veriqid_server.sh 2>/dev/null
chmod +x test_veriqid_server.sh

./test_veriqid_server.sh <ETHKEY>
```

### 8.4 End-to-end browser test

1. Open **http://localhost:8080** in Chrome
2. Install the browser extension from `extension/` (Chrome → `chrome://extensions` → Developer mode → Load unpacked → select the `extension/` folder)
3. Click the Veriqid extension icon and configure the bridge URL (`http://localhost:9090`) and key path
4. Click "Sign up with Veriqid" on KidsTube
5. The extension auto-detects the form, calls the bridge, generates proofs, and auto-fills the fields
6. Submit — registration should succeed

### 8.5 Parent Dashboard test

1. Open the parent dashboard at **http://localhost:8080/parent** (served by the Veriqid server)
2. Create a parent account
3. Add a child and verify them
4. Check the activity log — you should see the KidsTube registration
5. Test one-click revocation

---

## Restarting After a Shutdown

Ganache is ephemeral — all blockchain data is lost when it stops. After restarting:

1. Start Ganache: `ganache --port 7545`
2. Redeploy contracts: `cd contracts && truffle migrate --reset --network development` (copy new address)
3. Re-create identities using the new contract address
4. Restart bridge and server with the new contract address

Key files (`key1`, `key2`, etc.) persist on disk but are not registered on the new contract — you must re-register them or create new ones.

---

## Project Structure

```
Veriqid/
├── README.md                           # Project overview & VE-ASC improvements
├── SETUP.md                            # This file
├── go.mod / go.sum                     # Go module (github.com/patmekury/veriqid)
│
├── pkg/
│   ├── u2sso/                          # Core crypto (CGO → libsecp256k1 Boquila)
│   │   ├── u2ssolib.go                 # Ring signatures, key derivation, proofs
│   │   └── U2SSO.go                    # Smart contract Go bindings
│   └── ve_asc/                         # VE-ASC enhancements (pure Go)
│       ├── protocol.go                 # Setup, Gen, Prove, Verify, CompareProtocols
│       ├── nullifier.go                # 3-layer HMAC-SHA256 nullifier system
│       ├── attributes.go              # Pedersen commitments + ZK range proofs
│       ├── merkle.go                   # Merkle tree (1M identities) + sparse revocation
│       └── benchmark.go                # Original vs VE-ASC comparative benchmarks
│
├── bridge/bridge.go                    # Bridge API — HTTP wrapper around CGO crypto
├── cmd/
│   ├── bridge/main.go                  # Bridge entry point (port 9090)
│   ├── client/main.go                  # CLI tool (create, register, auth)
│   ├── server/main.go                  # Original minimal server (Phase 1)
│   ├── veriqid-server/main.go          # Production server (SQLite, sessions, VE-ASC)
│   └── demo-platform/main.go           # KidsTube demo server
│
├── contracts/
│   ├── U2SSO.sol                       # Original contract (reference)
│   ├── Veriqid.sol                     # Enhanced contract (owner fix, verifier registry)
│   └── contracts/VeriqidV2.sol         # VE-ASC contract (Merkle, epochs, nullifiers)
│
├── extension/                          # Chrome extension (Manifest V3)
│   ├── manifest.json
│   ├── background.js                   # Service worker — bridge comms
│   ├── content.js                      # Form detection + auto-fill
│   ├── popup.html / popup.js / popup.css
│   └── icons/
│
├── extension-portable/                 # Portable extension variant
│
├── dashboard/                          # Parent portal
│   ├── index.html
│   ├── dashboard.js
│   └── dashboard.css
│
├── demo-platform/                      # KidsTube demo
│   ├── templates/                      # HTML templates
│   └── static/                         # CSS, JS, images
│
├── internal/                           # Internal packages (session, store, etc.)
├── templates/                          # Server HTML templates
├── static/                             # Original web UI
├── crypto-dbpoe/                       # libsecp256k1 Boquila fork (copied from U2SSO/)
├── test_bridge.sh                      # Automated bridge API test suite (17 tests)
└── test_veriqid_server.sh              # Automated server test suite
```

---

## Troubleshooting

### libsecp256k1 build issues

**`autogen.sh` fails with "cannot execute: required file not found"**
Windows CRLF line endings. Fix with: `dos2unix $(find . -name "*.sh" -o -name "*.ac" -o -name "*.am")`

**`configure` says `--enable-module-ringcip` is unrecognized**
The Bug B fix (adding `AC_ARG_ENABLE(module_ringcip, ...)` to `configure.ac`) wasn't applied. See Step 3.

**`ENABLE_MODULE_RINGCIP` shows `#undef` after configure**
You ran `configure` before applying the bug fixes. Run `make clean`, re-apply fixes, then `./autogen.sh && ./configure --enable-module-ringcip --enable-experimental`.

### Smart contract deployment

**"Could not connect to your Ethereum client"**
Ganache isn't running or isn't on port 7545.

**"Could not find artifacts for U2SSO"**
The `.sol` files aren't in the Truffle `contracts/contracts/` subfolder. Run `cp *.sol contracts/` from within the `contracts/` directory.

### Bridge / Server

**Bridge crashes on bad input**
The original `u2ssolib.go` used `log.Fatal()` which kills the process. This has been fixed — all functions now return errors gracefully.

**"no contract found at address"**
Ganache was restarted (it's ephemeral). Redeploy with `truffle migrate --reset --network development`.

**`build output "bridge" already exists and is a directory`**
Use `go build -o bridge-server ./cmd/bridge/` — the `-o` flag avoids the naming conflict with the `bridge/` package directory.
