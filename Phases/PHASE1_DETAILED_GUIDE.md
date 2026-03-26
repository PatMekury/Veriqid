# Veriqid Phase 1: Environment Setup - Complete Detailed Guide

**Time estimate: ~2 hours**
**Goal: Get the crypto library building, smart contract deployed, and proof-of-concept running locally.**

---

## BEFORE YOU START: Understanding What You're Building

Veriqid uses a **C cryptographic library** (libsecp256k1 with Boquila extensions) that gets called from **Go** via **CGO** (Go's C interop). This is why the setup is more involved than a typical Go project. Here's what needs to happen:

1. The C library (`crypto-dbpoe/`) must be compiled and installed on your system
2. Go must be able to find and link against that C library at compile time
3. A local Ethereum blockchain (Ganache) must be running for the smart contract
4. The U2SSO smart contract must be deployed to that blockchain
5. At least 2 identities must be registered (ring signatures need a minimum ring size of 2)

**CRITICAL: You must use WSL2 (Windows Subsystem for Linux) or MSYS2 on Windows.**
The libsecp256k1 Boquila fork uses Unix autotools (`autogen.sh`, `configure`, `make`) which don't work in PowerShell or CMD.

---

## STEP 1: Install WSL2 (Windows Users Only)

**Time: ~10 minutes (plus possible restart)**

If you already have WSL2 with Ubuntu, skip to Step 2.

### 1.1 Open PowerShell as Administrator

- Press `Win + X`, select "Terminal (Admin)" or "PowerShell (Admin)"

### 1.2 Install WSL2

```powershell
wsl --install
```

This installs WSL2 with Ubuntu by default. If it says WSL is already installed:

```powershell
wsl --install -d Ubuntu-22.04
```

### 1.3 Restart your computer if prompted

### 1.4 Set up Ubuntu

After restart, Ubuntu will open automatically (or search "Ubuntu" in Start menu):

- Choose a username (e.g., `patrick`)
- Choose a password (you'll need this for `sudo` commands)
- Wait for setup to complete

### 1.5 Verify WSL2 is running

```bash
# Inside the Ubuntu terminal:
cat /etc/os-release
# Should show Ubuntu 22.04 or similar

uname -r
# Should show something like 5.15.x.x-microsoft-standard-WSL2
```

---

## STEP 2: Install Go 1.22+

**Time: ~5 minutes**

All commands from here are run **inside the WSL2 Ubuntu terminal**.

### 2.1 Remove any old Go installation

```bash
sudo rm -rf /usr/local/go
```

### 2.2 Download Go 1.22.5

```bash
cd ~
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
```

You should see a download progress bar. The file is about 67MB.

### 2.3 Extract to /usr/local

```bash
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
```

### 2.4 Add Go to your PATH

```bash
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 2.5 Verify installation

```bash
go version
```

**Expected output:** `go version go1.22.5 linux/amd64`

If it says `go: command not found`, close and reopen your terminal, then try again.

### 2.6 Verify CGO is enabled (it should be by default)

```bash
go env CGO_ENABLED
```

**Expected output:** `1`

If it shows `0`, enable it:

```bash
export CGO_ENABLED=1
echo 'export CGO_ENABLED=1' >> ~/.bashrc
```

### 2.7 Clean up the download

```bash
rm ~/go1.22.5.linux-amd64.tar.gz
```

---

## STEP 3: Install Build Tools and OpenSSL Development Headers

**Time: ~5 minutes**

### 3.1 Update your package lists

```bash
sudo apt update && sudo apt upgrade -y
```

This may take a few minutes. Enter your password when prompted.

### 3.2 Install C/C++ build essentials

```bash
sudo apt install -y build-essential
```

This installs: `gcc`, `g++`, `make`, `libc-dev`, and other essential build tools.

### 3.3 Install autotools (needed by libsecp256k1's build system)

```bash
sudo apt install -y autoconf automake libtool
```

- **autoconf** - generates the `configure` script from `configure.ac`
- **automake** - generates `Makefile.in` from `Makefile.am`
- **libtool** - handles shared library creation across platforms

### 3.4 Install OpenSSL development headers

```bash
sudo apt install -y libssl-dev
```

This installs the header files (`openssl/rand.h`, etc.) that the C code includes. Without this, CGO compilation will fail with `fatal error: openssl/rand.h: No such file or directory`.

### 3.5 Install pkg-config (helps the compiler find libraries)

```bash
sudo apt install -y pkg-config
```

### 3.6 Verify everything is installed

```bash
gcc --version
# Should show gcc (Ubuntu ...) 11.x or 12.x

autoconf --version
# Should show autoconf (GNU Autoconf) 2.71

openssl version
# Should show OpenSSL 3.x.x

ls /usr/include/openssl/rand.h
# Should show the file path (not "No such file")
```

---

## STEP 4: Install Node.js 18+

**Time: ~3 minutes**

Node.js is needed for Truffle (smart contract deployment) and Ganache (local blockchain).

### 4.1 Add NodeSource repository

```bash
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
```

### 4.2 Install Node.js

```bash
sudo apt install -y nodejs
```

### 4.3 Verify installation

```bash
node --version
# Expected: v18.x.x (any 18+ version is fine)

npm --version
# Expected: 9.x.x or 10.x.x
```

---

## STEP 5: Install Ganache and Truffle

**Time: ~3 minutes**

### 5.1 Install Ganache CLI globally

```bash
sudo npm install -g ganache
```

**IMPORTANT: Use `sudo`** — without it you'll get `EACCES: permission denied` because global npm packages install to `/usr/lib/node_modules/` which requires root access on Ubuntu.

Ganache creates a local Ethereum blockchain with 10 pre-funded test accounts. Each account gets 1000 fake ETH for testing.

### 5.2 Install Truffle globally

```bash
sudo npm install -g truffle
```

Truffle compiles Solidity contracts and deploys them to the blockchain.

### 5.3 Verify installations

```bash
ganache --version
# Expected: ganache v7.x.x (any version is fine)

truffle version
# Expected output like:
# Truffle v5.x.x
# Solidity - 0.8.13 (solc-js)
# Node v18.x.x
```

---

## STEP 6: Navigate to the Veriqid Project Folder

**Time: ~2 minutes**

### 6.1 Navigate to the Veriqid directory from WSL2

WSL2 accesses your Windows drives via `/mnt/c/`, `/mnt/d/`, etc.

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"
```

**If your folder is named "Shape Rotor" (with a space):**

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape Rotor/U2SSO/Veriqid"
```

**WARNING:** Spaces in paths can break some Go/C tooling. If you run into issues, consider renaming the folder on Windows to `Shape_Rotor` (with underscore).

### 6.2 Verify you're in the right place

```bash
ls
```

**Expected output:**

```
SETUP.md  bridge  cmd  contracts  dashboard  demo-platform  extension  go.mod  go.sum  pkg  sdk  static
```

### 6.3 Verify the file structure

```bash
ls cmd/client/
# Expected: main.go

ls cmd/server/
# Expected: main.go

ls pkg/u2sso/
# Expected: U2SSO.go  u2ssolib.go

ls contracts/
# Expected: U2SSO.sol  migrations  truffle-config.js

ls static/
# Expected: index.html, signup.html, login.html, CSS files, images
```

---

## STEP 7: Copy crypto-dbpoe from the U2SSO Repo

**Time: ~2 minutes**

The `crypto-dbpoe/` directory contains the **custom fork of libsecp256k1** with Boquila extensions for ring signatures and anonymous credentials. This is the core cryptographic engine.

### 7.1 Copy the entire crypto-dbpoe directory

```bash
# Make sure you're in the Veriqid folder
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"

# Copy from the parent U2SSO repo
cp -r ../crypto-dbpoe ./crypto-dbpoe
```

### 7.2 Verify the copy

```bash
ls crypto-dbpoe/
# Expected: autogen.sh, configure.ac, src/, include/, Makefile.am, etc.

ls crypto-dbpoe/include/
# Expected: secp256k1.h, secp256k1_ringcip.h, and other .h files
```

The key files that matter:

- `include/secp256k1.h` - Main library header
- `include/secp256k1_ringcip.h` - Ring signature + Boquila extensions header
- `src/` - All the C source code
- `autogen.sh` - Script that generates the build system
- `configure.ac` - Build configuration template

### 7.3 CRITICAL: Fix Windows Line Endings (CRLF → LF)

Because the files were stored on a Windows filesystem, they have Windows-style line endings (`\r\n`). Linux can't execute scripts with `\r` in them — you'll get `cannot execute: required file not found` even though the file clearly exists.

**Install dos2unix and convert all build files at once:**

```bash
sudo apt install -y dos2unix

cd crypto-dbpoe
find . -type f \( -name "*.sh" -o -name "*.ac" -o -name "*.am" -o -name "*.m4" -o -name "Makefile*" -o -name "*.in" \) -exec dos2unix {} +
```

**What this does:** Strips the `\r` (carriage return) from every line in all build-related files. Without this, `./autogen.sh` will fail with `cannot execute: required file not found` because Linux tries to find `/bin/sh\r` (with the invisible `\r`) as the interpreter.

**Alternative if dos2unix isn't available:**

```bash
sed -i 's/\r$//' autogen.sh
chmod +x autogen.sh
```

But `dos2unix` on the whole directory is the safer approach since many files need fixing.

---

## STEP 8: Build libsecp256k1 with Boquila Extensions

**Time: ~10-15 minutes**

This is the **most critical step**. If this fails, nothing else will work.

### 8.1 Enter the crypto-dbpoe directory

```bash
cd crypto-dbpoe
```

### 8.2 Fix two known bugs in the build configuration

The `crypto-dbpoe` fork has two bugs that prevent building on modern systems. Both must be fixed before running `autogen.sh`.

**Bug A: `libtoolize` macro conflict**

`configure.ac` uses `AC_CONFIG_MACRO_DIR` (singular) but modern `libtoolize` requires `AC_CONFIG_MACRO_DIRS` (plural), and it conflicts with `ACLOCAL_AMFLAGS` in `Makefile.am`.

```bash
# Fix configure.ac: singular → plural
sed -i 's/AC_CONFIG_MACRO_DIR(\[build-aux\/m4\])/AC_CONFIG_MACRO_DIRS([build-aux\/m4])/' configure.ac

# Fix Makefile.am: remove conflicting line (AC_CONFIG_MACRO_DIRS handles it now)
sed -i '/^ACLOCAL_AMFLAGS/d' Makefile.am
```

**Bug B: Missing `--enable-module-ringcip` flag definition**

The `configure.ac` file checks for `$enable_module_ringcip` but never defines the `AC_ARG_ENABLE` block that creates the `--enable-module-ringcip` CLI flag. Without this fix, the flag is silently ignored and the ringcip module won't be compiled.

```bash
# Add the missing AC_ARG_ENABLE block after the aggsig block
sed -i '/\[enable_module_aggsig=no\])/a\
\
AC_ARG_ENABLE(module_ringcip,\
    AS_HELP_STRING([--enable-module-ringcip],[enable ring signature CIP module]),\
    [enable_module_ringcip=$enableval],\
    [enable_module_ringcip=no])' configure.ac
```

**Verify both fixes were applied:**

```bash
grep "AC_CONFIG_MACRO_DIRS" configure.ac
# Expected: AC_CONFIG_MACRO_DIRS([build-aux/m4])

grep "AC_ARG_ENABLE(module_ringcip" configure.ac
# Expected: AC_ARG_ENABLE(module_ringcip,
```

If the `sed` command for Bug B didn't work (this can happen with CRLF issues), you can manually edit `configure.ac` and add these 4 lines right after the `enable_module_aggsig=no])` line:

```
AC_ARG_ENABLE(module_ringcip,
    AS_HELP_STRING([--enable-module-ringcip],[enable ring signature CIP module]),
    [enable_module_ringcip=$enableval],
    [enable_module_ringcip=no])
```

### 8.3 Run autogen.sh to generate build scripts

```bash
./autogen.sh
```

**What this does:** Reads `configure.ac` and `Makefile.am`, then generates the `configure` script and `Makefile.in` template.

**Expected output:** Several lines ending with something like:

```
libtoolize: putting auxiliary files in 'build-aux'.
...
configure.ac:...: installing 'build-aux/missing'
```

You'll see some harmless warnings about `AC_PROG_CC_C99` being obsolete and `bench_generator_LDFLAGS` being defined twice — these are safe to ignore.

**If it fails with "Permission denied":**

```bash
chmod +x autogen.sh
./autogen.sh
```

**If it fails with "autoreconf: not found":**

```bash
sudo apt install -y autoconf automake libtool
./autogen.sh
```

**If it fails with "AC_CONFIG_MACRO_DIRS conflicts with ACLOCAL_AMFLAGS":**

You missed Bug A above. Run the two `sed` commands for Bug A, then re-run `./autogen.sh`.

### 8.4 Run configure with the required flags

```bash
./configure --enable-module-ringcip --enable-experimental
```

**What the flags mean:**

- `--enable-module-ringcip` - Enables the ring signature and Boquila key derivation modules. **Without this flag, the `secp256k1_ringcip.h` functions won't be compiled**, and CGO will fail with "undefined reference" errors.
- `--enable-experimental` - Required because the ringcip module is marked as experimental in the codebase.

**Expected output:** Many lines of checks, ending with a summary that includes:

```
configure: WARNING: experimental build
configure: Building aggsig module: no
configure: Building aggsig module: yes    <-- This second "aggsig" line is actually ringcip
```

**NOTE:** The summary says "Building aggsig module: yes" twice — the second one is actually the ringcip module. The developers reused the log message label in `configure.ac`. What matters is that there are TWO "aggsig" lines and the second one says `yes`.

**CRITICAL CHECK — Verify ringcip is actually defined in the config header:**

```bash
grep "ENABLE_MODULE_RINGCIP" src/libsecp256k1-config.h
```

**Must show:** `#define ENABLE_MODULE_RINGCIP 1`

**If it shows `#undef ENABLE_MODULE_RINGCIP`:** The module was NOT enabled. This happens if you ran `configure` before applying the Bug B fix (Step 8.2). Fix it by cleaning and reconfiguring:

```bash
make clean
./configure --enable-module-ringcip --enable-experimental
# Then re-check: grep "ENABLE_MODULE_RINGCIP" src/libsecp256k1-config.h
```

**If the `--enable-module-ringcip` flag is "unrecognized"** (warning at the end of configure output): The Bug B fix from Step 8.2 wasn't applied. Go back and add the `AC_ARG_ENABLE` block, then re-run `./autogen.sh` followed by `./configure ...`.

**If it fails with "configure: error: ...":** Read the error message carefully. Common issues:

- Missing `gcc`: `sudo apt install -y build-essential`
- Missing `libtool`: `sudo apt install -y libtool`

### 8.5 Compile the library

```bash
make
```

**What this does:** Compiles all the C source files into a shared library (`libsecp256k1.so`) and a static library (`libsecp256k1.a`).

**Expected output:** Many lines of compilation, ending without errors:

```
  CC       src/libsecp256k1_la-secp256k1.lo
  CCLD     libsecp256k1.la
```

**If it fails:** Read the error. Most common issue is missing dependencies from Step 3.

### 8.6 Install the library system-wide

```bash
sudo make install
```

**What this does:** Copies the compiled library files and headers to system directories:

- Libraries go to `/usr/local/lib/` (`libsecp256k1.so`, `libsecp256k1.a`)
- Headers go to `/usr/local/include/` (`secp256k1.h`, `secp256k1_ringcip.h`)

This is what makes the `#cgo LDFLAGS: -lsecp256k1` line in the Go code work - the linker can find `-lsecp256k1` because it's now in a standard library path.

**Verify `secp256k1_ringcip.h` appears in the install output.** You should see a line like:

```
/usr/bin/install -c -m 644 include/secp256k1.h include/secp256k1_generator.h include/secp256k1_commitment.h include/secp256k1_ringcip.h '/usr/local/include'
```

If `secp256k1_ringcip.h` is NOT in that list, the ringcip module wasn't compiled. Go back to Step 8.4 and verify the `#define` in the config header.

### 8.7 Update the dynamic linker cache

```bash
sudo ldconfig
```

**What this does:** Tells the system's dynamic linker about the newly installed library so programs can find `libsecp256k1.so` at runtime.

### 8.8 Verify the installation

```bash
# Check library files exist
ls /usr/local/lib/libsecp256k1*
# Expected: libsecp256k1.a, libsecp256k1.la, libsecp256k1.so, libsecp256k1.so.2, etc.

# Check header files exist
ls /usr/local/include/secp256k1.h
ls /usr/local/include/secp256k1_ringcip.h
# Both should exist

# Check the linker can find it
ldconfig -p | grep secp256k1
# Expected: libsecp256k1.so.2 (libc6,x86-64) => /usr/local/lib/libsecp256k1.so.2
```

### 8.9 Return to the Veriqid directory

The installed library is **permanent** — it persists across terminal sessions, directory changes, and reboots. You only need to rebuild it if you change the C source code.

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"
```

---

## STEP 9: Verify Go + CGO Compilation

**Time: ~5 minutes**

This is your **key checkpoint**. If `go build` succeeds here, your entire toolchain is working.

### 9.1 Download Go dependencies

```bash
go mod tidy
```

**What this does:** Downloads all the Go packages listed in `go.mod` (primarily `go-ethereum`). This may take a minute the first time.

**Expected output:** Possibly some "go: downloading ..." lines, then it returns to the prompt without errors.

### 9.2 Try building the client

```bash
go build ./cmd/client/
```

**What happens behind the scenes:**

1. Go reads `cmd/client/main.go`
2. It sees the `import "C"` block with CGO directives
3. It invokes `gcc` to compile the C parts, linking against `-lcrypto` (OpenSSL) and `-lsecp256k1` (our library)
4. It compiles the Go parts
5. It links everything into a single binary

**Expected output:** No output = success! A binary named `client` appears in the current directory.

### 9.3 Try building the server

```bash
go build ./cmd/server/
```

**Expected output:** No output = success! A binary named `server` appears.

### 9.4 Clean up the binaries (optional)

```bash
rm -f client server
```

### 9.5 Troubleshooting CGO errors

| Error Message | Cause | Fix |
|---------------|-------|-----|
| `fatal error: secp256k1.h: No such file or directory` | Headers not installed | Re-run `sudo make install` in crypto-dbpoe/ |
| `fatal error: secp256k1_ringcip.h: No such file or directory` | Same as above | Same fix |
| `fatal error: openssl/rand.h: No such file or directory` | OpenSSL dev headers missing | `sudo apt install -y libssl-dev` |
| `/usr/bin/ld: cannot find -lsecp256k1` | Library not in linker path | Run `sudo ldconfig` after make install |
| `undefined reference to 'secp256k1_ringcip_context_create'` | Library built without ringcip module | Rebuild: `./configure --enable-module-ringcip --enable-experimental && make && sudo make install` |
| `undefined reference to 'RAND_bytes'` | OpenSSL not linked | Verify `-lcrypto` is in the CGO LDFLAGS |

---

## STEP 10: Start Ganache (Local Ethereum Blockchain)

**Time: ~2 minutes**

### 10.1 Open a NEW terminal tab/window

You need Ganache running in its own terminal because it stays running in the foreground.

**Option A - New WSL2 tab (recommended):** Right-click the Ubuntu tab in Windows Terminal, select "Duplicate Tab". This gives you a second WSL2 shell.

**Option B - New window:** Open a new Ubuntu terminal from the Start menu

**Option C - Windows terminal:** You can also run Ganache in a regular Windows CMD/PowerShell if you have Node.js installed on Windows too. The Go server in WSL2 connects via `http://127.0.0.1:7545` either way since WSL2 and Windows share localhost.

### 10.2 Start Ganache on port 7545

```bash
ganache --port 7545
```

### 10.3 Read and save the output

Ganache will print something like this:

```
ganache v7.9.2 (@ganache/cli: 0.10.2, @ganache/core: 0.10.2)
Starting RPC server

Available Accounts
==================
(0) 0xAbC123...def456 (1000 ETH)
(1) 0x789Ghi...jkl012 (1000 ETH)
(2) 0x345Mno...pqr678 (1000 ETH)
...

Private Keys
==================
(0) 0xabc123def456789...  <-- YOU NEED THIS
(1) 0x789ghi012jkl345...
(2) 0x345mno678pqr901...
...

RPC Listening on 127.0.0.1:7545
```

### 10.4 SAVE THESE VALUES (you'll need them soon)

Write down or copy:

1. **First account address:** e.g., `0xAbC123...def456`
2. **First private key:** e.g., `abc123def456789...` (remove the `0x` prefix!)

**IMPORTANT:** The private key is used WITHOUT the `0x` prefix when passed to the client CLI.

### 10.5 Leave this terminal running

Ganache must stay running for the rest of the setup. Don't close this terminal.

---

## STEP 11: Deploy the U2SSO Smart Contract

**Time: ~5 minutes**

Go back to your **original terminal** (not the Ganache one).

### 11.1 Navigate to the contracts directory

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts"
```

### 11.2 Set up the Truffle directory structure

Truffle expects a specific directory layout. When you run `truffle migrate` from a directory, it looks for a `contracts/` subfolder *within* that directory for Solidity files. Since our project already has `U2SSO.sol` at the top level of `contracts/`, we need to create the nested structure:

```bash
# Create the contracts subfolder that Truffle expects
mkdir -p contracts

# Copy U2SSO.sol into it (Truffle looks for contracts/contracts/U2SSO.sol)
cp U2SSO.sol contracts/
```

Your directory should now look like:

```
Veriqid/contracts/
├── truffle-config.js
├── U2SSO.sol              ← original copy (kept for reference)
├── contracts/
│   └── U2SSO.sol          ← copy Truffle will compile
├── migrations/
│   └── 1_deploy_contracts.js
└── package.json
```

### 11.3 Initialize npm and deploy the contract

```bash
npm init -y
npm install

truffle migrate --network development
```

**What this does:**

1. Truffle reads `truffle-config.js` and connects to Ganache at `127.0.0.1:7545`
2. It compiles `U2SSO.sol` using the Solidity 0.8.13 compiler
3. It runs `migrations/1_deploy_contracts.js` which calls `deployer.deploy(U2SSO)`
4. The compiled contract bytecode is sent as a transaction to Ganache
5. Ganache mines the block and returns the contract address

**Expected output:**

```
Compiling your contracts...
===========================
> Compiling ./contracts/U2SSO.sol
> Artifacts written to ./build/contracts
> Compiled successfully using:
   - solc: 0.8.13+commit.abaa5c0e.Emscripten.clang

Starting migrations...
======================

1_deploy_contracts.js
=====================
   Deploying 'U2SSO'
   ------------------
   > transaction hash:    0x...
   > contract address:    0xYourContractAddress  <-- SAVE THIS!
   > block number:        1
   > gas used:            ...
   > ...

   > Saving artifacts
   > Total cost: ...

Summary
=======
> Total deployments:   1
> Final cost:          ...
```

### 11.4 SAVE THE CONTRACT ADDRESS

Copy the **contract address** from the output. It looks like: `0x1234567890AbCdEf1234567890AbCdEf12345678`

You'll use this in every subsequent command.

### 11.5 Troubleshooting deployment

| Error | Fix |
|-------|-----|
| `Could not connect to your Ethereum client` | Make sure Ganache is running on port 7545 |
| `Could not find artifacts for U2SSO from any sources` | U2SSO.sol is not in the `contracts/contracts/` subfolder. Run `cp U2SSO.sol contracts/` from within the `contracts/` directory |
| `Everything is up to date, there is nothing to compile` | Same as above — Truffle can't find any `.sol` files in its expected `contracts/` subfolder |
| `Error: CompileError` | Check U2SSO.sol syntax |
| `Migrations directory not found` | Make sure you're in the `contracts/` directory |

---

## STEP 12: Create Test Identities (Minimum 2)

**Time: ~5 minutes**

You need at least 2 registered identities because ring signatures require a ring of at least 2 public keys.

### 12.1 Navigate back to the Veriqid root

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"
```

### 12.2 Create Identity #1

Replace `<CONTRACT_ADDRESS>` with your contract address from Step 11, and `<PRIVATE_KEY>` with the Ganache private key from Step 10 (WITHOUT the `0x` prefix).

```bash
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -ethkey <PRIVATE_KEY_WITHOUT_0x> \
  -command create \
  -keypath ./key1
```

**Example with real-looking values:**

```bash
go run ./cmd/client \
  -contract 0x5FbDB2315678afecb367f032d93F642f64180aa3 \
  -ethkey ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
  -command create \
  -keypath ./key1
```

**What happens behind the scenes:**

1. `CreatePasskey("./key1")` - Generates 32 random bytes via OpenSSL's `RAND_bytes()`, saves to file `./key1`
2. `LoadPasskey("./key1")` - Reads back the 32-byte master secret key (msk)
3. `CreateID(mskBytes)` - Calls `secp256k1_boquila_gen_mpk()` to derive the 33-byte compressed master public key (mpk) from the msk
4. `AddIDstoIdR(...)` - Sends an Ethereum transaction calling `addID(uint256, uint)` on the smart contract, storing the mpk on-chain

**Expected output:**

```
No client address was given. Taking default: http://127.0.0.1:7545
Found the contract at 0x5FbDB...
Current id size: 0
Passkey created successfully
Added ID to index: 0
```

### 12.3 Verify key1 was created

```bash
ls -la key1
# Expected: -rw-r--r-- ... 32 ... key1  (32 bytes)

xxd key1
# Shows 32 bytes of random hex data (your master secret key)
```

**IMPORTANT:** This `key1` file IS the master secret key. In production, this would be stored securely (encrypted, in a hardware key, etc.). For testing, it's just a plain file.

### 12.4 Create Identity #2

Use the **same `-ethkey`** but a **different `-keypath`**:

```bash
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -ethkey <PRIVATE_KEY_WITHOUT_0x> \
  -command create \
  -keypath ./key2
```

**Understanding the two keys:**

- **`-ethkey`** = Your Ethereum wallet private key (from Ganache). This just pays the gas fee for the on-chain transaction. Use the **same key** every time — it's like a credit card paying for registration.
- **`-keypath ./key2`** = Where to save the NEW identity's **master secret key (msk)**. This is what makes the identity unique — 32 fresh random bytes generated by `RAND_bytes()`, completely different from key1. This is the Veriqid identity itself.

Both key1 and key2 are funded by the same Ganache wallet, but they are **completely separate identities** with different master keys, different public keys, and different service-specific pseudonyms.

**Expected output:**

```
No client address was given. Taking default: http://127.0.0.1:7545
Found the contract at 0x5FbDB...
Current id size: 1
Passkey created successfully
Added ID to index: 1
```

Note that "Current id size" is now `1` (one identity was already registered), and the new one is added at index `1`.

### 12.5 (Optional) Create a 3rd identity for a larger ring

```bash
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -ethkey <PRIVATE_KEY_WITHOUT_0x> \
  -command create \
  -keypath ./key3
```

More identities = larger ring = better anonymity (but 2 is the minimum for testing).

### 12.6 Verify identity count on the contract

```bash
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -command load \
  -keypath ./key1
```

**Expected output:**

```
Found the contract at 0x...
Current id size: 2  <-- Should be 2 or more
Passkey loaded successfully, length: 32
```

---

## STEP 13: Start the Veriqid Server

**Time: ~2 minutes**

### 13.1 Make sure you're in the Veriqid root directory

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"
```

### 13.2 Start the server

```bash
go run ./cmd/server -contract 0x<CONTRACT_ADDRESS>
```

**Expected output:**

```
No client address was given. Taking default: http://127.0.0.1:7545
Found the contract at 0x5FbDB...
Current id size: 2
Veriqid server started at http://localhost:8080
```

The server is now running and:

- Serving static HTML files from `./static/`
- Listening for signup requests at `/signup`
- Listening for login requests at `/login`
- Generating challenges for signup at `/directSignup`
- Generating challenges for login at `/directLogin`

### 13.3 Leave this terminal running

The server stays in the foreground. You'll need another terminal for the next step.

---

## STEP 14: Test in the Browser

**Time: ~5 minutes**

### 14.1 Open the web UI

Open your browser (on Windows) and go to:

```
http://localhost:8080
```

**What you should see:** The U2SSO welcome page with a logo and two buttons: "Log in" and "Sign up".

### 14.2 Test the Sign Up flow

1. Click **"Sign up"**
2. You'll see a form with:
   - **Challenge:** A pre-filled hex string (64 characters = 32 bytes in hex). This is generated fresh by the server using `CreateChallenge()` which calls `RAND_bytes()`.
   - **Service name:** A pre-filled hex string. This is the SHA-256 hash of the service name `"abc_service"`.
   - Empty fields for: User name, Public key (SPK), Decoy size, Nullifier, Membership Proof

3. **Copy the Challenge and Service Name values** - you'll need them in the next step.

### 14.3 Generate a registration proof via CLI

Open a **third terminal** (leave Ganache and the server running in their terminals).

```bash
cd "/mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid"

go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -command register \
  -keypath ./key1 \
  -sname <SERVICE_NAME_HEX_FROM_FORM> \
  -challenge <CHALLENGE_HEX_FROM_FORM>
```

**What happens behind the scenes:**

1. Loads the master secret key from `./key1`
2. Derives a service-specific secret key (ssk) via `secp256k1_boquila_derive_ssk()` using the service name
3. Derives a service-specific public key (spk) via `secp256k1_boquila_derive_spk()`
4. Creates a 65-byte Boquila authentication proof via `secp256k1_boquila_auth_prove()`
5. Fetches ALL active identities from the smart contract
6. Creates a ring membership proof via `secp256k1_boquila_prove_memmpk()` that proves "my mpk is in the on-chain set" WITHOUT revealing WHICH mpk

**Expected output (three important values):**

```
Total ID size: 2
Chosen ring size: 2 and m: 1
Proof hex: a1b2c3d4e5f6...  <-- LONG hex string (the membership proof)
SPK hex: 02abc123def456...  <-- 66 characters (33 bytes = compressed secp256k1 point)
N: 2
```

### 14.4 Fill in the signup form

Go back to the browser and fill in:

- **User name:** anything (e.g., "alice")
- **Public key for the account:** paste the `SPK hex` value
- **Decoy size:** paste the `N` value (e.g., `2`)
- **Nullifier:** leave empty (nullifiers aren't implemented in the C/Go path)
- **Membership Proof:** paste the `Proof hex` value

Click **"SignUp!"**

### 14.5 Verify registration success

If the proof is valid, you should see a **"Registration Successful"** page. If it fails, you'll see a failure page - double-check that you copied the values correctly and that the challenge matches.

### 14.6 Test the Login flow

1. Go back to `http://localhost:8080`
2. Click **"Log in"**
3. You'll see a similar form with a new challenge and the same service name
4. Generate an auth proof via CLI:

```bash
go run ./cmd/client \
  -contract 0x<CONTRACT_ADDRESS> \
  -command auth \
  -keypath ./key1 \
  -sname <SERVICE_NAME_HEX_FROM_LOGIN_FORM> \
  -challenge <CHALLENGE_HEX_FROM_LOGIN_FORM>
```

5. Fill in the login form with the SPK and signature, and submit

---

## STEP 15: Phase 1 Completion Checklist

Run through this checklist to confirm everything is working:

```
[ ] WSL2 installed and running Ubuntu
[ ] Go 1.22+ installed (go version shows 1.22.x)
[ ] CGO enabled (go env CGO_ENABLED shows 1)
[ ] build-essential, autoconf, automake, libtool installed
[ ] libssl-dev installed (openssl/rand.h exists)
[ ] Node.js 18+ installed
[ ] Ganache installed and running on port 7545
[ ] Truffle installed
[ ] crypto-dbpoe/ copied into Veriqid/
[ ] libsecp256k1 built with --enable-module-ringcip --enable-experimental
[ ] libsecp256k1 installed (files in /usr/local/lib/ and /usr/local/include/)
[ ] ldconfig run (linker can find libsecp256k1)
[ ] go build ./cmd/client/ succeeds
[ ] go build ./cmd/server/ succeeds
[ ] U2SSO.sol deployed to Ganache (contract address saved)
[ ] 2+ identities created (key1, key2 files exist)
[ ] Server running at localhost:8080
[ ] Web UI loads in browser
[ ] (Optional) Full signup/login flow works end-to-end
```

---

## What Each File Does (Reference)

### cmd/client/main.go
The CLI tool. Supports 4 commands:
- `create` - Generates a 32-byte master secret key (msk), derives master public key (mpk), registers mpk on the smart contract
- `load` - Reads an existing msk from a file
- `register` - Generates a ring membership proof + service-specific public key (spk) for a given service
- `auth` - Generates a 65-byte Boquila authentication proof for a given service

### cmd/server/main.go
The web server. Serves HTML forms and handles signup/login verification:
- `/` - Serves static files (index.html, etc.)
- `/directSignup` - Generates a fresh challenge, injects it into the signup form
- `/directLogin` - Generates a fresh challenge, injects it into the login form
- `/signup` - Receives proof, spk, challenge from form. Calls `RegistrationVerify()` to check the ring membership proof
- `/login` - Receives signature, spk, challenge. Calls `AuthVerify()` to check the Boquila auth proof, and checks that the spk was previously registered

### pkg/u2sso/u2ssolib.go
The core cryptographic library (Go wrapper around C via CGO):
- `CreateChallenge()` - 32 random bytes via OpenSSL
- `CreatePasskey()` - Generates and saves 32-byte msk
- `LoadPasskey()` - Reads msk from file
- `CreateID()` - Derives 33-byte compressed mpk from msk using `secp256k1_boquila_gen_mpk()`
- `RegistrationProof()` - Creates ring membership proof + spk + auth proof
- `RegistrationVerify()` - Verifies a ring membership proof
- `AuthProof()` - Creates a 65-byte Boquila authentication proof
- `AuthVerify()` - Verifies an authentication proof
- `AddIDstoIdR()` - Registers an mpk on the smart contract
- `GetallActiveIDfromContract()` - Fetches all active IDs from the contract (used to build the ring)

**Key constants:**
- `gen_seed_fix = 11` - The ring CIP context seed (32 bytes of 0x0B). ANY implementation must use this exact value.
- `N = 2` - Ring base (binary)
- `M = 10` - Max ring exponent (ring size up to 2^10 = 1024)

### pkg/u2sso/U2SSO.go
Auto-generated Go bindings for the U2SSO.sol smart contract (created by `abigen`). Provides Go functions that map to Solidity functions: `AddID()`, `GetIDSize()`, `GetIDs()`, `GetState()`, `RevokeID()`, `GetIDIndex()`.

### contracts/U2SSO.sol
The Solidity smart contract. Stores identities as `{uint256 id, uint id33, bool active, address owner}`. Has a known bug: `addID()` uses the deployer's address as owner for ALL identities instead of `msg.sender`. This will be fixed in Phase 3 with `Veriqid.sol`.

---

## Next Steps: Phase 2 - Bridge API

With Phase 1 complete, you have a working but manual flow (CLI generates proofs, human copies them into web forms). Phase 2 creates `bridge/bridge.go` which is an HTTP JSON API that automates this - the browser extension (Phase 5) will call this API directly instead of requiring CLI copy-paste.
