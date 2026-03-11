# Veriqid - Phase 1: Environment Setup Guide

**Estimated time: ~2 hours**
**Goal: Get the crypto library building and proof-of-concept running locally.**

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.22+ | Server, client, CGO bindings |
| Node.js | 18+ | Truffle, Ganache, future SNARK path |
| Ganache | Latest | Local Ethereum blockchain |
| Truffle | Latest | Smart contract deployment |
| OpenSSL dev headers | - | Required by libsecp256k1 CGO |
| WSL2 or MSYS2 | - | **Windows only** - CGO compilation |

---

## Step-by-Step Instructions

### Step 1: Install System Dependencies (~15 min)

**On WSL2/Ubuntu (RECOMMENDED for Windows):**

```bash
# Update packages
sudo apt update && sudo apt upgrade -y

# Install Go 1.22+
sudo rm -rf /usr/local/go
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
go version  # Should show 1.22+

# Install build essentials + OpenSSL dev headers
sudo apt install -y build-essential autoconf automake libtool libssl-dev pkg-config

# Install Node.js 18+
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt install -y nodejs
node --version  # Should show 18+

# Install Ganache CLI and Truffle
npm install -g ganache truffle
```

**Verify installations:**
```bash
go version          # go1.22.x or higher
node --version      # v18.x or higher
ganache --version   # Should output version
truffle version     # Should show Truffle + Solidity versions
```

### Step 2: Access the Veriqid Folder (~2 min)

**From WSL2**, mount your Windows folder:

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"
```

> **NOTE:** If your folder is named "Shape Rotor" (with a space), some tools may break.
> Consider renaming it to `Shape_Rotor` on Windows first.

### Step 3: Copy crypto-dbpoe from the U2SSO Repo (~2 min)

The `crypto-dbpoe/` directory contains the custom libsecp256k1 fork with Boquila extensions.
It must be copied into the Veriqid folder:

```bash
# From the Veriqid directory:
cp -r ../crypto-dbpoe ./crypto-dbpoe
```

This gives you:
```
Veriqid/
  crypto-dbpoe/
    autogen.sh
    configure.ac
    src/
    include/
    ...
```

### Step 4: Build libsecp256k1 with Boquila Extensions (~15 min)

This is the most critical step. The library provides ring signatures and Boquila key derivation.

```bash
cd crypto-dbpoe

# Generate build scripts
./autogen.sh

# Configure with ring signature + experimental modules enabled
./configure --enable-module-ringcip --enable-experimental

# Build
make

# Install system-wide (makes -lsecp256k1 available to CGO)
sudo make install

# Update library cache
sudo ldconfig
```

**Verify the build:**
```bash
ls /usr/local/lib/libsecp256k1*
# Should show: libsecp256k1.so, libsecp256k1.a, etc.

ls /usr/local/include/secp256k1*
# Should show: secp256k1.h, secp256k1_ringcip.h, etc.
```

**If `./autogen.sh` fails:**
```bash
sudo apt install autoconf automake libtool
./autogen.sh
```

**If `make` fails with missing headers:**
```bash
sudo apt install libssl-dev
```

### Step 5: Verify Go + CGO Compilation (~5 min)

Test that Go can compile with the C library:

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

# Set CGO to enabled (should be default on Linux/WSL)
export CGO_ENABLED=1

# Try building the server (the real test)
go build ./cmd/server/

# If this succeeds without errors, your environment is GOOD
```

**Common CGO errors and fixes:**

| Error | Fix |
|-------|-----|
| `secp256k1.h: No such file` | Run `sudo make install` in crypto-dbpoe/ |
| `cannot find -lsecp256k1` | Run `sudo ldconfig` after make install |
| `undefined reference to secp256k1_ringcip_*` | Rebuild with `--enable-module-ringcip --enable-experimental` |
| `openssl/rand.h: No such file` | `sudo apt install libssl-dev` |

### Step 6: Start Ganache (~2 min)

Open a **separate terminal** (can be Windows CMD or WSL):

```bash
ganache --port 7545
```

This starts a local Ethereum blockchain. You'll see 10 test accounts with private keys.
**Save the first private key** - you'll need it for the next step.

Example output:
```
Available Accounts
==================
(0) 0x... (1000 ETH)

Private Keys
==================
(0) 0xabc123...  <-- COPY THIS (without the 0x prefix)
```

### Step 7: Deploy the Smart Contract (~5 min)

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts

# Deploy U2SSO.sol to Ganache
truffle migrate --network development
```

The output will show the deployed contract address:
```
Deploying 'U2SSO'
-----------------
> contract address:    0x1234567890abcdef...  <-- SAVE THIS
```

**Save this contract address** - you'll use it in every subsequent command.

### Step 8: Create Test Identities (~5 min)

You need **at least 2 identities** for ring signatures to work (ring size must be >= 2).

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

# Create identity 1
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -ethkey <GANACHE_PRIVATE_KEY_WITHOUT_0x> \
  -command create \
  -keypath ./key1

# Create identity 2
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -ethkey <GANACHE_PRIVATE_KEY_WITHOUT_0x> \
  -command create \
  -keypath ./key2
```

Each command should output:
```
Found the contract at 0x...
Current id size: 0  (then 1 for the second)
Passkey created successfully
Added ID to index: 0  (then 1)
```

### Step 9: Start the Veriqid Server (~2 min)

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

go run ./cmd/server -contract 0x<CONTRACT_ADDRESS>
```

Output:
```
Found the contract at 0x...
Current id size: 2
Veriqid server started at http://localhost:8080
```

### Step 10: Test in Browser (~5 min)

Open **http://localhost:8080** in your browser.

1. **Test Signup:** Click "Sign Up" - the form shows a pre-generated challenge and service name
2. **Test Login:** Click "Log In" - similar form for authentication

> **Note:** For the signup/login to actually WORK end-to-end, you need to run the
> client CLI to generate proofs and paste them into the forms. This is the manual
> workflow that the Bridge API (Phase 2) will automate.

**Manual end-to-end test:**

```bash
# In a new terminal, register identity 1 with the service
# Use the challenge and service name shown on the signup page

go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -command register \
  -keypath ./key1 \
  -sname <SERVICE_NAME_HEX_FROM_FORM> \
  -challenge <CHALLENGE_HEX_FROM_FORM>

# Copy the output proof, spk, and N into the signup form fields
```

---

## Phase 1 Completion Checklist

- [ ] Go 1.22+ installed with CGO enabled
- [ ] libsecp256k1 built with Boquila extensions (`--enable-module-ringcip`)
- [ ] OpenSSL dev headers installed
- [ ] `go build ./cmd/server/` compiles successfully
- [ ] Ganache running on port 7545
- [ ] U2SSO.sol deployed via Truffle
- [ ] 2+ identities created via client CLI
- [ ] Server running at localhost:8080
- [ ] Web UI loads in browser

---

## Folder Structure After Phase 1

```
Veriqid/
├── go.mod                          # Module: github.com/patmekury/veriqid
├── go.sum                          # Dependency checksums
├── SETUP.md                        # This file
├── cmd/
│   ├── client/main.go              # CLI client (from clientapp.go)
│   └── server/main.go              # Web server (from server.go)
├── bridge/bridge.go                # Phase 2: Bridge API (placeholder)
├── pkg/u2sso/
│   ├── u2ssolib.go                 # Core crypto library (CGO bindings)
│   └── U2SSO.go                    # Smart contract Go bindings
├── contracts/
│   ├── U2SSO.sol                   # Original smart contract
│   ├── truffle-config.js           # Truffle deployment config
│   └── migrations/
│       └── 1_deploy_contracts.js   # Deployment migration
├── crypto-dbpoe/                   # Copied from repo - libsecp256k1 fork
├── sdk/                            # Phase 8: JS SDK (placeholder)
├── extension/                      # Phase 5: Browser extension (placeholder)
├── dashboard/                      # Phase 6: Parent dashboard (placeholder)
├── demo-platform/                  # Phase 7: Demo platform (placeholder)
├── static/                         # Web UI HTML/CSS/images
│   ├── index.html
│   ├── signup.html
│   ├── login.html
│   ├── registration_success.html
│   ├── login_success.html
│   ├── login_fail.html
│   └── ... (CSS + images)
├── key1                            # Generated: identity 1 passkey
└── key2                            # Generated: identity 2 passkey
```

---

## Next: Phase 2 - Bridge API

Once Phase 1 is working, proceed to create `bridge/bridge.go` which wraps the
CGO crypto library into a clean HTTP JSON API that the browser extension can call.
