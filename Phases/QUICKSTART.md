# Veriqid Demo — Quick Start Guide

**Everything you need to run the full end-to-end demo from scratch.**

You need **5 terminals** (or WSL tabs). All commands assume you're starting from the Veriqid project root:

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
```

---

## Terminal 1 — Ganache (Local Blockchain)

```bash
ganache --port 7545
```

Leave this running. Copy the first private key from the output (without the `0x` prefix). You'll need it for the other terminals.

**Example output:**

```
Available Accounts
==================
(0) 0x742186E2C8719A1d543b171FC6C03D848D1A4714 (1000 ETH)

Private Keys
==================
(0) 0x35a04a77f3ae8ace9a5005e5d1e7324d0b7fce9351265302ac97bcb1cedb3828
```

Your private key (strip the `0x`):

```
35a04a77f3ae8ace9a5005e5d1e7324d0b7fce9351265302ac97bcb1cedb3828
```

---

## Terminal 2 — Deploy Contract

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle migrate --reset --network development
```

Two contracts deploy — **U2SSO** (block 1) and **Veriqid** (block 2). Copy the **Veriqid** contract address (the second one).

**Save these two values — you'll use them everywhere:**

| Value | Example |
|-------|---------|
| `CONTRACT` | `0x34b597c707aC36886a9175401BA9993e8E888Db2` |
| `ETHKEY` | `35a04a77f3ae8ace9a5005e5d1e7324d0b7fce9351265302ac97bcb1cedb3828` |

> **Important:** Always use the **Veriqid** contract address (block 2), not the U2SSO one (block 1). The Veriqid contract has the `onlyVerifier` modifier, age brackets, and batch retrieval that the demo requires.

---

## Terminal 2 (reuse) — Bridge Server

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go build -o bridge-server ./cmd/bridge/
./bridge-server -contract <CONTRACT> -client http://127.0.0.1:7545
```

**Example:**

```bash
go build -o bridge-server ./cmd/bridge/
./bridge-server -contract 0x34b597c707aC36886a9175401BA9993e8E888Db2 -client http://127.0.0.1:7545
```

You should see:

```
===========================================
  Veriqid Bridge API
===========================================
  Listening:  http://127.0.0.1:9090
  Contract:   0x34b597c707aC36886a9175401BA9993e8E888Db2
  RPC:        http://127.0.0.1:7545
```

> **Important:** Always rebuild with `go build` before starting the bridge, especially after code changes. The bridge uses `NewKeyedTransactorWithChainID` for EIP-155 signing — if you skip the rebuild, on-chain transactions will silently revert because `msg.sender` gets recovered as the wrong address.

---

## Terminal 3 (new) — Register Dummy Identities

Ring membership proofs require **at least 2 identities** on-chain. Create dummy identities before anything else:

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
mkdir -p keys

curl -s -X POST http://127.0.0.1:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{"keypath":"keys/dummy1","ethkey":"<ETHKEY>","age_bracket":1}'

curl -s -X POST http://127.0.0.1:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{"keypath":"keys/dummy2","ethkey":"<ETHKEY>","age_bracket":1}'
```

**Example (with real key):**

```bash
curl -s -X POST http://127.0.0.1:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{"keypath":"keys/dummy1","ethkey":"35a04a77f3ae8ace9a5005e5d1e7324d0b7fce9351265302ac97bcb1cedb3828","age_bracket":1}'

curl -s -X POST http://127.0.0.1:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{"keypath":"keys/dummy2","ethkey":"35a04a77f3ae8ace9a5005e5d1e7324d0b7fce9351265302ac97bcb1cedb3828","age_bracket":1}'
```

Both should return `"success": true` with `"index": 0` and `"index": 1`. If you get `"index": -1`, see Troubleshooting below.

---

## Terminal 3 (reuse) — Veriqid Server (Parent Dashboard)

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go run ./cmd/veriqid-server \
  -contract <CONTRACT> \
  -ethkey <ETHKEY> \
  -service "KidsTube" \
  -port 8080 \
  -master-secret "my-hackathon-secret"
```

**Example:**

```bash
go run ./cmd/veriqid-server \
  -contract 0x34b597c707aC36886a9175401BA9993e8E888Db2 \
  -ethkey 35a04a77f3ae8ace9a5005e5d1e7324d0b7fce9351265302ac97bcb1cedb3828 \
  -service "KidsTube" \
  -port 8080 \
  -master-secret "my-hackathon-secret"
```

You should see: `Veriqid server running on :8080`

**Dashboard URL:** http://localhost:8080/parent

---

## Terminal 4 — KidsTube Server

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go run ./cmd/demo-platform \
  -contract <CONTRACT> \
  -client http://127.0.0.1:7545 \
  -port 3000 \
  -service "KidsTube" \
  -basedir . \
  -veriqid-server http://127.0.0.1:8080
```

**Example:**

```bash
go run ./cmd/demo-platform \
  -contract 0x34b597c707aC36886a9175401BA9993e8E888Db2 \
  -client http://127.0.0.1:7545 \
  -port 3000 \
  -service "KidsTube" \
  -basedir . \
  -veriqid-server http://127.0.0.1:8080
```

You should see: `KidsTube Demo Platform — http://localhost:3000`

**KidsTube URL:** http://localhost:3000

---

## Running the Demo

### Overview of running services

| Terminal | Service | Port | URL |
|----------|---------|------|-----|
| 1 | Ganache | 7545 | — |
| 2 | Bridge | 9090 | — |
| 3 | Veriqid (Parent Dashboard) | 8080 | http://localhost:8080/parent |
| 4 | KidsTube | 3000 | http://localhost:3000 |

### Step-by-step demo flow

**1. Parent creates account**

- Open http://localhost:8080/parent
- Create a parent account (email + password)

**2. Parent adds a child**

- Click "Add Child"
- Enter child's name (e.g., "Aled") and age bracket (e.g., "Under 13")
- Click "Verification Approval" to approve the child
- The server registers the child's identity on-chain (you'll see the log in Terminal 3)
- **Copy the 12-word mnemonic phrase** — this is displayed once

**3. Child sets up the portable extension**

- In Chrome, go to `chrome://extensions`
- Enable Developer Mode
- Click "Load unpacked" → select the `extension-portable` folder
- Click the Veriqid extension icon
- Paste the 12-word mnemonic phrase
- Click "Save Key"
- The badge should turn green ("ON") once the bridge is detected

**4. Child signs up on KidsTube**

- Open http://localhost:3000
- Click "Sign up with Veriqid"
- The extension auto-detects the form and fills in the proof (~1 second)
- The status turns green: "Veriqid identity verified!"
- Type a username (e.g., "AledPlays")
- Click "Create Account"
- You're redirected to the video grid: "Welcome, AledPlays!"

**5. Parent checks the dashboard**

- Go back to http://localhost:8080/parent
- The activity log shows the child registered on KidsTube

**6. Test login**

- On KidsTube, click "Log Out"
- Click "Log In"
- The extension auto-fills the auth proof
- You're logged back in

**7. Parent revokes access**

- On the parent dashboard, click "Revoke" on the child's identity card
- Confirm with password

**8. Child loses access**

- Refresh KidsTube → "Access Revoked — Your Veriqid identity has been revoked by your parent"

---

## Resetting Everything

If you need to start completely fresh:

```bash
# 1. Stop all servers (Ctrl+C in each terminal)

# 2. Delete databases and keys
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
rm -f kidstube.db veriqid.db veriqid.db-shm veriqid.db-wal
rm -rf keys

# 3. Restart Ganache (Terminal 1)
ganache --port 7545
# Copy the new account (0) private key (without 0x prefix)

# 4. Redeploy contract (new terminal)
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle migrate --reset --network development
# Copy the NEW Veriqid contract address (the second one, block 2)

# 5. Rebuild and restart bridge with new values
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
go build -o bridge-server ./cmd/bridge/
./bridge-server -contract <NEW_CONTRACT> -client http://127.0.0.1:7545

# 6. Re-register dummy identities with new ethkey
mkdir -p keys
curl -s -X POST http://127.0.0.1:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{"keypath":"keys/dummy1","ethkey":"<NEW_ETHKEY>","age_bracket":1}'
curl -s -X POST http://127.0.0.1:9090/api/identity/create \
  -H "Content-Type: application/json" \
  -d '{"keypath":"keys/dummy2","ethkey":"<NEW_ETHKEY>","age_bracket":1}'

# 7. Restart Veriqid server and KidsTube with new values
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `"index": -1` when creating identities | The on-chain transaction is silently reverting. Make sure you rebuilt the bridge with `go build -o bridge-server ./cmd/bridge/` before starting it. The bridge requires `NewKeyedTransactorWithChainID` for correct EIP-155 signing. Also verify you're using the **Veriqid** contract address (block 2), not U2SSO (block 1). |
| `"index": -1` after rebuild | Nuke everything and start fresh: stop all servers, delete DBs + keys, restart Ganache, redeploy, rebuild bridge, and re-register dummies. Previous failed transactions may have corrupted nonces. |
| "Identity verification failed. The proof is invalid." | Make sure at least 2 identities are on-chain with `"index": 0` and `"index": 1`. Re-register dummies if you restarted Ganache. |
| "Challenge already used (replay attack prevented)" | Refresh the signup/login page to get a new challenge. |
| "Missing auth data. Is the Veriqid extension installed?" | Check the extension is loaded, the bridge is running, and you've pasted a valid mnemonic. |
| Extension badge shows "OFF" (red) | Bridge isn't running on port 9090. Start it. |
| "No account found for this identity" on login | You signed up with a different mnemonic. Delete `kidstube.db` and re-signup. |
| Everything fails after Ganache restart | Ganache is ephemeral. Redeploy contract, re-register dummies, restart all servers with new contract address, delete all `.db` files. |
| Veriqid server won't start | Make sure you're passing `-ethkey`. It's required for on-chain registration. |
| Parent dashboard "Home" goes to KidsTube | This was fixed — the nav now links to `/parent` instead of `/`. Hard-refresh the page. |
