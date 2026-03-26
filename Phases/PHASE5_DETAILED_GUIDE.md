# Veriqid Phase 5: Browser Extension — Complete Detailed Guide

**Time estimate: ~6 hours**
**Goal: Build a Chrome extension (Manifest V3) that manages the user's master secret key, detects Veriqid-enabled signup/login forms, communicates with the local bridge API to generate proofs, and auto-fills form fields — eliminating the manual CLI copy-paste workflow. Additionally, redesign ALL web-facing pages to use the Veriqid brand design system.**

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
> 4. **Start the Veriqid server** (Terminal 3):
>    ```bash
>    cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
>    go run ./cmd/veriqid-server \
>      -contract 0x<NEW_CONTRACT_ADDRESS> \
>      -service "KidsTube" \
>      -port 8080
>    ```
> 5. **Re-create identities** — old key files exist on disk but are not registered on the new contract. Either create fresh keys via the bridge API or re-run the test script.
>
> You will also need to use one of the Ganache private keys (without the `0x` prefix) for any operations that send Ethereum transactions (like creating identities or authorizing verifiers).

---

## BEFORE YOU START: Understanding What You're Building

In Phases 1–4, generating proofs required either:
- **Phase 1 (manual):** Copy challenge from browser → paste into CLI → copy proof output → paste back into browser form
- **Phase 2–4 (semi-automated):** Use `curl` to call the bridge API, then copy-paste the JSON response values into the form

Phase 5 eliminates ALL manual steps. The browser extension:

1. **Detects** Veriqid-enabled forms on any website (by looking for hidden fields with specific IDs like `veriqid-challenge`, `veriqid-service-name`)
2. **Reads** the challenge and service name directly from the page
3. **Calls** the local bridge API (`http://localhost:9090`) to generate proofs
4. **Auto-fills** the form fields with proof values
5. **Optionally submits** the form automatically

```
Phase 1 (manual):       Browser → Human → CLI → CGO/C → Human → Browser
Phase 2 (curl):         Browser → Human → curl → Bridge → CGO/C → Human → Browser
Phase 5 (automated):    Browser Extension → Bridge API → CGO/C → Extension → Form Submit
```

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│  CHROME EXTENSION (Manifest V3)                         │
│                                                         │
│  ┌──────────────┐  ┌───────────────┐  ┌──────────────┐ │
│  │  Popup UI    │  │  Background   │  │  Content     │ │
│  │  (popup.html │  │  Service      │  │  Script      │ │
│  │   popup.js)  │  │  Worker       │  │  (content.js)│ │
│  │              │  │  (background  │  │              │ │
│  │  - Setup     │  │   .js)        │  │  - Detect    │ │
│  │  - Identity  │  │              │  │    forms     │ │
│  │    list      │  │  - Bridge     │  │  - Auto-fill │ │
│  │  - Status    │  │    comm      │  │  - Show      │ │
│  │  - Parent    │  │  - Storage    │  │    badge     │ │
│  │    controls  │  │  - Key mgmt   │  │  - Inject UI │ │
│  └──────┬───────┘  └──────┬────────┘  └──────┬───────┘ │
│         │                 │                   │         │
│         └─────────────────┼───────────────────┘         │
│              chrome.runtime.sendMessage                  │
└─────────────────────────────────┬───────────────────────┘
                                  │
                                  │ HTTP (localhost:9090)
                                  ▼
                     ┌────────────────────────┐
                     │  BRIDGE API (Go)       │
                     │  - /api/identity/create│
                     │  - /api/identity/      │
                     │    register            │
                     │  - /api/identity/auth  │
                     │  - /api/status         │
                     └────────────────────────┘
```

### Component Responsibilities

| Component | File(s) | Purpose |
|-----------|---------|---------|
| **Manifest** | `manifest.json` | Extension metadata, permissions, CSP |
| **Popup UI** | `popup.html`, `popup.js`, `popup.css` | User-facing UI for setup, identity management, parent controls |
| **Background Service Worker** | `background.js` | Bridge API communication, key management, encrypted storage, badge updates |
| **Content Script** | `content.js` | DOM scanning for Veriqid forms, auto-fill, badge injection on detected pages |
| **Icons** | `icons/` | Extension icon at 16, 48, 128px |

### Why a Service Worker (Not a Background Page)?

Manifest V3 (required for new Chrome extensions) replaced persistent background pages with **service workers**. Key differences:
- Service workers are **ephemeral** — Chrome starts them on-demand and kills them after ~30 seconds of inactivity
- No DOM access — can't use `document` or `window` in the service worker
- State must be stored in `chrome.storage` (not in-memory variables)
- Communication via `chrome.runtime.sendMessage()` / `chrome.runtime.onMessage`

This means the extension must persist all state (keypath, bridge URL, setup status) to `chrome.storage.local`, not to JavaScript variables.

---

## THE VERIQID DESIGN SYSTEM

**All web-facing pages built in Phase 5 (and the redesigned Phase 4 templates) follow this design system.** This section defines the exact colors, typography, spacing, and component styles used everywhere.

### Color Palette

```css
:root {
  /* ---- Primary Colors ---- */
  --vq-dark:         #0A1F2F;    /* Deep teal — primary background      */
  --vq-dark-mid:     #0F2B3C;    /* Slightly lighter teal — cards/nav   */
  --vq-dark-light:   #153A4F;    /* Lighter teal — hover states, inputs */
  --vq-accent:       #B8F12A;    /* Lime/chartreuse — CTAs, highlights  */
  --vq-accent-hover: #A3D925;    /* Darker lime — hover on accent       */

  /* ---- Neutrals ---- */
  --vq-white:        #FFFFFF;
  --vq-off-white:    #F0F4F8;    /* Page background for light sections  */
  --vq-gray-100:     #E2E8F0;
  --vq-gray-200:     #CBD5E1;
  --vq-gray-300:     #94A3B8;
  --vq-gray-400:     #64748B;
  --vq-gray-500:     #475569;

  /* ---- Semantic ---- */
  --vq-success:      #4ADE80;
  --vq-error:        #F87171;
  --vq-warning:      #FBBF24;
  --vq-info:         #60A5FA;

  /* ---- Shadows ---- */
  --vq-shadow-sm:    0 1px 3px rgba(0,0,0,0.12);
  --vq-shadow-md:    0 4px 12px rgba(0,0,0,0.15);
  --vq-shadow-lg:    0 8px 30px rgba(0,0,0,0.2);
  --vq-shadow-card:  0 4px 20px rgba(10,31,47,0.3);

  /* ---- Border Radius ---- */
  --vq-radius-sm:    8px;
  --vq-radius-md:    12px;
  --vq-radius-lg:    16px;
  --vq-radius-xl:    24px;
  --vq-radius-full:  9999px;

  /* ---- Typography ---- */
  --vq-font-family:  'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  --vq-font-mono:    'SF Mono', 'Fira Code', 'Consolas', monospace;
}
```

### Typography Scale

| Element | Size | Weight | Color |
|---------|------|--------|-------|
| **Hero heading** | 48px / 3rem | 700 (bold) | `--vq-white` |
| **Section heading** | 28px / 1.75rem | 700 | `--vq-white` or `--vq-dark` |
| **Card heading** | 18px / 1.125rem | 600 (semibold) | `--vq-white` or `--vq-dark` |
| **Body text** | 15px / 0.9375rem | 400 (regular) | `--vq-gray-200` (on dark) or `--vq-gray-500` (on light) |
| **Small/help text** | 13px / 0.8125rem | 400 | `--vq-gray-300` (on dark) or `--vq-gray-400` (on light) |
| **Accent keyword** | Same as context | Same | `--vq-accent` |
| **Mono/code** | 13px | 400 | `--vq-font-mono` |

### Button Styles

**Primary button (lime CTA — NOT glassy):**
```css
.vq-btn-primary {
  background: var(--vq-accent);
  color: var(--vq-dark);
  font-weight: 600;
  font-size: 15px;
  padding: 12px 28px;
  border: none;
  border-radius: var(--vq-radius-full);   /* Pill shape */
  cursor: pointer;
  transition: background 0.2s, transform 0.1s;
}
.vq-btn-primary:hover {
  background: var(--vq-accent-hover);
  transform: translateY(-1px);
}
```

**Secondary button (outline on dark):**
```css
.vq-btn-secondary {
  background: transparent;
  color: var(--vq-white);
  font-weight: 500;
  font-size: 14px;
  padding: 10px 24px;
  border: 1.5px solid var(--vq-gray-300);
  border-radius: var(--vq-radius-full);
  cursor: pointer;
  transition: border-color 0.2s, color 0.2s;
}
.vq-btn-secondary:hover {
  border-color: var(--vq-accent);
  color: var(--vq-accent);
}
```

### Card Styles

**Stat badge card (floating, dark bg):**
```css
.vq-stat-badge {
  background: var(--vq-dark-mid);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: var(--vq-radius-md);
  padding: 14px 18px;
  color: var(--vq-white);
  box-shadow: var(--vq-shadow-card);
}
.vq-stat-badge .stat-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--vq-accent);
}
```

**Feature card (white bg, bottom section):**
```css
.vq-feature-card {
  background: var(--vq-white);
  border-radius: var(--vq-radius-lg);
  padding: 28px 24px;
  box-shadow: var(--vq-shadow-sm);
  transition: box-shadow 0.2s, transform 0.2s;
}
.vq-feature-card:hover {
  box-shadow: var(--vq-shadow-md);
  transform: translateY(-2px);
}
.vq-feature-card.highlighted {
  background: var(--vq-accent);
  color: var(--vq-dark);
}
```

### Form Inputs

```css
.vq-input {
  width: 100%;
  padding: 12px 16px;
  background: var(--vq-dark-light);
  border: 1.5px solid rgba(255,255,255,0.1);
  border-radius: var(--vq-radius-sm);
  color: var(--vq-white);
  font-size: 14px;
  font-family: var(--vq-font-family);
  transition: border-color 0.2s, box-shadow 0.2s;
}
.vq-input::placeholder { color: var(--vq-gray-400); }
.vq-input:focus {
  outline: none;
  border-color: var(--vq-accent);
  box-shadow: 0 0 0 3px rgba(184,241,42,0.15);
}
```

### Navigation Bar

```css
.vq-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 40px;
  background: transparent;   /* Sits on top of --vq-dark hero */
}
.vq-nav-link {
  color: var(--vq-gray-200);
  text-decoration: none;
  font-size: 15px;
  font-weight: 500;
  padding: 4px 0;
  transition: color 0.2s;
}
.vq-nav-link:hover, .vq-nav-link.active {
  color: var(--vq-white);
}
.vq-nav-link.active {
  border-bottom: 2px solid var(--vq-white);
}
```

---

## PREREQUISITES

Before starting Phase 5, you must have Phases 1–4 fully complete:

```
[x] libsecp256k1 built and installed with --enable-module-ringcip
[x] go build ./cmd/client/ succeeds (CGO works)
[x] bridge/bridge.go compiles (go build -o bridge-server ./cmd/bridge/)
[x] Veriqid.sol deployed and Go bindings regenerated (Phase 3)
[x] cmd/veriqid-server compiles (go build -o veriqid-server ./cmd/veriqid-server/)
[x] Ganache running on port 7545
[x] Bridge API running on port 9090 and tested (Phase 2)
[x] Veriqid server running on port 8080 (Phase 4)
[x] 2+ identities registered on the contract
```

You'll also need:
- **Google Chrome** (or Chromium-based browser like Edge, Brave)
- A text editor for writing JavaScript/HTML/CSS
- The Ganache private key and contract address from your current session

---

## STEP 1: Understand the Form Field Convention

**Time: ~10 minutes (reading, no coding)**

### 1.1 How the extension detects Veriqid forms

In Phase 4, the Veriqid server's signup and login HTML templates include **hidden form fields** with specific IDs that the extension scans for. These are the "hooks" that tell the extension "this page has a Veriqid form."

**Signup form hidden fields:**

```html
<input type="hidden" id="veriqid-challenge" name="challenge" value="{{.Challenge}}">
<input type="hidden" id="veriqid-service-name" name="sname" value="{{.ServiceName}}">
<input type="text" id="veriqid-spk" name="spk">
<input type="text" id="veriqid-ring-size" name="n">
<input type="text" id="veriqid-proof" name="proof">
```

**Login form hidden fields:**

```html
<input type="hidden" id="veriqid-challenge" name="challenge" value="{{.Challenge}}">
<input type="hidden" id="veriqid-service-name" name="sname" value="{{.ServiceName}}">
<input type="text" id="veriqid-spk" name="spk">
<input type="text" id="veriqid-auth-proof" name="signature">
```

### 1.2 Detection strategy

1. Look for `document.getElementById('veriqid-challenge')` — if it exists, this page has a Veriqid form
2. Check for `veriqid-proof` (signup) OR `veriqid-auth-proof` (login) to determine the form type
3. Read the `value` attribute of `veriqid-challenge` and `veriqid-service-name`

### 1.3 The auto-fill flow

**Signup:**
```
Content script detects veriqid-challenge + veriqid-proof → "signup form"
  → Sends message to background: {action: "generateRegistrationProof", challenge, serviceName}
  → Background calls bridge: POST /api/identity/register
  → Bridge returns: {proof_hex, spk_hex, ring_size}
  → Content script fills: veriqid-spk, veriqid-proof, veriqid-ring-size
```

**Login:**
```
Content script detects veriqid-challenge + veriqid-auth-proof → "login form"
  → Sends message to background: {action: "generateAuthProof", challenge, serviceName}
  → Background calls bridge: POST /api/identity/auth
  → Content script fills: veriqid-auth-proof, veriqid-spk
```

---

## STEP 2: Create the Extension File Structure

**Time: ~5 minutes**

### 2.1 Create directories

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

mkdir -p extension/icons
```

### 2.2 Target file structure

```
Veriqid/
└── extension/
    ├── manifest.json
    ├── background.js
    ├── content.js
    ├── popup.html
    ├── popup.js
    ├── popup.css
    └── icons/
        ├── icon16.png
        ├── icon48.png
        └── icon128.png
```

---

## STEP 3: Write the Manifest (manifest.json)

**Time: ~10 minutes**

### 3.1 Create `extension/manifest.json`

```json
{
  "manifest_version": 3,
  "name": "Veriqid",
  "version": "0.5.0",
  "description": "Privacy-first children's internet identity — verify once, play everywhere.",

  "permissions": [
    "storage",
    "activeTab",
    "scripting"
  ],

  "host_permissions": [
    "http://localhost:9090/*",
    "http://127.0.0.1:9090/*"
  ],

  "background": {
    "service_worker": "background.js"
  },

  "content_scripts": [
    {
      "matches": ["<all_urls>"],
      "js": ["content.js"],
      "run_at": "document_idle"
    }
  ],

  "action": {
    "default_popup": "popup.html",
    "default_icon": {
      "16": "icons/icon16.png",
      "48": "icons/icon48.png",
      "128": "icons/icon128.png"
    }
  },

  "icons": {
    "16": "icons/icon16.png",
    "48": "icons/icon48.png",
    "128": "icons/icon128.png"
  }
}
```

### 3.2 Permission explanations

| Permission | Why |
|-----------|-----|
| `storage` | Persist keypath, bridge URL, setup status across service worker restarts |
| `activeTab` | Interact with the current tab's DOM |
| `scripting` | Programmatically inject scripts if needed |
| `host_permissions: localhost:9090` | Allow fetch requests to the bridge API |

---

## STEP 4: Create Placeholder Icons

**Time: ~5 minutes**

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/extension

python3 -c "
import struct, zlib

def create_png(w, h, r, g, b, filename):
    def chunk(ct, data):
        c = ct + data
        return struct.pack('>I', len(data)) + c + struct.pack('>I', zlib.crc32(c) & 0xffffffff)
    hdr = b'\x89PNG\r\n\x1a\n'
    ihdr = chunk(b'IHDR', struct.pack('>IIBBBBB', w, h, 8, 2, 0, 0, 0))
    raw = b''
    for y in range(h):
        raw += b'\x00'
        for x in range(w):
            raw += bytes([r, g, b])
    idat = chunk(b'IDAT', zlib.compress(raw))
    iend = chunk(b'IEND', b'')
    with open(filename, 'wb') as f:
        f.write(hdr + ihdr + idat + iend)

# Veriqid brand: dark teal with lime accent (icon is lime on teal)
create_png(16, 16, 184, 241, 42, 'icons/icon16.png')
create_png(48, 48, 184, 241, 42, 'icons/icon48.png')
create_png(128, 128, 184, 241, 42, 'icons/icon128.png')
print('Icons created (lime green #B8F12A)')
"
```

---

## STEP 5: Write the Background Service Worker (background.js)

**Time: ~45 minutes**

### 5.1 Create `extension/background.js`

```javascript
// ============================================================
// Veriqid Background Service Worker (Manifest V3)
// ============================================================

const DEFAULT_BRIDGE_URL = "http://127.0.0.1:9090";

// ---- Storage Helpers ----

async function getConfig() {
  const result = await chrome.storage.local.get([
    "bridgeUrl", "keypath", "isSetup", "ethkey", "autoFill", "autoSubmit"
  ]);
  return {
    bridgeUrl: result.bridgeUrl || DEFAULT_BRIDGE_URL,
    keypath: result.keypath || "",
    isSetup: result.isSetup || false,
    ethkey: result.ethkey || "",
    autoFill: result.autoFill !== undefined ? result.autoFill : true,
    autoSubmit: result.autoSubmit !== undefined ? result.autoSubmit : false,
  };
}

async function saveConfig(config) {
  await chrome.storage.local.set(config);
}

// ---- Bridge Communication ----

async function bridgeFetch(endpoint, options = {}) {
  const config = await getConfig();
  const url = `${config.bridgeUrl}${endpoint}`;

  try {
    const response = await fetch(url, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...(options.headers || {}),
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(`Bridge returned ${response.status}: ${errorBody}`);
    }

    return await response.json();
  } catch (error) {
    if (error.message.includes("Failed to fetch") || error.message.includes("NetworkError")) {
      throw new Error("Bridge is not running. Start the bridge server on port 9090.");
    }
    throw error;
  }
}

async function checkBridgeStatus() {
  try {
    const status = await bridgeFetch("/api/status");
    await chrome.action.setBadgeText({ text: "ON" });
    await chrome.action.setBadgeBackgroundColor({ color: "#B8F12A" });
    return { connected: true, ...status };
  } catch (error) {
    await chrome.action.setBadgeText({ text: "OFF" });
    await chrome.action.setBadgeBackgroundColor({ color: "#F87171" });
    return { connected: false, error: error.message };
  }
}

// ---- Identity Operations ----

async function createIdentity(keypath, ethkey) {
  return await bridgeFetch("/api/identity/create", {
    method: "POST",
    body: JSON.stringify({ keypath, ethkey }),
  });
}

async function generateRegistrationProof(keypath, serviceName, challenge) {
  return await bridgeFetch("/api/identity/register", {
    method: "POST",
    body: JSON.stringify({
      keypath,
      service_name: serviceName,
      challenge,
    }),
  });
}

async function generateAuthProof(keypath, serviceName, challenge) {
  return await bridgeFetch("/api/identity/auth", {
    method: "POST",
    body: JSON.stringify({
      keypath,
      service_name: serviceName,
      challenge,
    }),
  });
}

// ---- Message Handler ----

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  handleMessage(message, sender)
    .then(sendResponse)
    .catch((error) => sendResponse({ success: false, error: error.message }));
  return true; // Keep channel open for async response
});

async function handleMessage(message, sender) {
  switch (message.action) {

    case "getConfig":
      return await getConfig();

    case "saveConfig":
      await saveConfig(message.config);
      return { success: true };

    case "checkBridge":
      return await checkBridgeStatus();

    case "createIdentity":
      return await createIdentity(message.keypath, message.ethkey);

    case "generateRegistrationProof": {
      const config = await getConfig();
      if (!config.keypath) {
        throw new Error("No identity configured. Open the Veriqid popup and complete setup.");
      }
      return await generateRegistrationProof(
        config.keypath,
        message.serviceName,
        message.challenge
      );
    }

    case "generateAuthProof": {
      const config = await getConfig();
      if (!config.keypath) {
        throw new Error("No identity configured. Open the Veriqid popup and complete setup.");
      }
      const authResult = await generateAuthProof(
        config.keypath,
        message.serviceName,
        message.challenge
      );
      // Also derive the SPK — auth endpoint doesn't return it
      let spkHex = "";
      try {
        const regResult = await generateRegistrationProof(
          config.keypath,
          message.serviceName,
          message.challenge
        );
        spkHex = regResult.spk_hex || "";
      } catch (e) {
        console.warn("Could not derive SPK for auth flow:", e.message);
      }
      return { ...authResult, spk_hex: spkHex };
    }

    case "pageHasVeriqidForm":
      await chrome.action.setBadgeText({ text: "V", tabId: sender.tab?.id });
      await chrome.action.setBadgeBackgroundColor({ color: "#B8F12A", tabId: sender.tab?.id });
      return { success: true };

    default:
      throw new Error(`Unknown action: ${message.action}`);
  }
}

// ---- Startup & Health Checks ----

chrome.runtime.onInstalled.addListener(async () => { await checkBridgeStatus(); });
chrome.runtime.onStartup.addListener(async () => { await checkBridgeStatus(); });

chrome.alarms.create("bridgeHealthCheck", { periodInMinutes: 1 });
chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name === "bridgeHealthCheck") await checkBridgeStatus();
});
```

### 5.2 Key implementation details

**`return true` in `onMessage.addListener`:** Chrome's message passing is synchronous by default. Returning `true` keeps the channel open for async `sendResponse`.

**`chrome.alarms` for health checks:** Service workers are ephemeral — `setInterval` doesn't survive restarts. `chrome.alarms` is the Manifest V3 way to schedule recurring tasks.

**SPK derivation during auth flow:** `AuthProof()` in Go returns only the proof hex, not the SPK. The login form needs the SPK to identify the user. The workaround is to also call the register endpoint to derive the SPK. In production, add a dedicated `/api/identity/derive-spk` bridge endpoint.

---

## STEP 6: Write the Content Script (content.js)

**Time: ~45 minutes**

### 6.1 Create `extension/content.js`

This content script uses the **Veriqid design system** for the floating badge — dark teal background, lime accent, pill-shaped buttons.

```javascript
// ============================================================
// Veriqid Content Script — Form Detection & Auto-Fill
// ============================================================

(function () {
  "use strict";

  // ---- Veriqid Design Tokens (inline — no CSS import in content scripts) ----
  const VQ = {
    dark:       "#0A1F2F",
    darkMid:    "#0F2B3C",
    accent:     "#B8F12A",
    accentHover:"#A3D925",
    white:      "#FFFFFF",
    gray300:    "#94A3B8",
    error:      "#F87171",
    success:    "#4ADE80",
    warning:    "#FBBF24",
    radiusMd:   "12px",
    radiusFull: "9999px",
    shadowLg:   "0 8px 30px rgba(0,0,0,0.25)",
    font:       "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
  };

  // ---- Form Detection ----

  function detectVeriqidForm() {
    const challengeEl = document.getElementById("veriqid-challenge");
    const serviceNameEl = document.getElementById("veriqid-service-name");

    if (!challengeEl || !serviceNameEl) return null;

    const challenge = challengeEl.value;
    const serviceName = serviceNameEl.value;
    if (!challenge || !serviceName) return null;

    const proofEl = document.getElementById("veriqid-proof");
    const authProofEl = document.getElementById("veriqid-auth-proof");

    if (proofEl) {
      return {
        type: "signup",
        challenge, serviceName,
        fields: {
          spk: document.getElementById("veriqid-spk"),
          proof: proofEl,
          ringSize: document.getElementById("veriqid-ring-size"),
        },
      };
    } else if (authProofEl) {
      return {
        type: "login",
        challenge, serviceName,
        fields: {
          spk: document.getElementById("veriqid-spk"),
          authProof: authProofEl,
        },
      };
    }

    return null;
  }

  // ---- Badge UI (Veriqid Design System) ----

  function injectVeriqidBadge(formInfo) {
    if (document.getElementById("veriqid-badge")) return;

    const badge = document.createElement("div");
    badge.id = "veriqid-badge";
    badge.innerHTML = `
      <div id="veriqid-badge-inner" style="
        position: fixed;
        bottom: 24px;
        right: 24px;
        background: ${VQ.dark};
        border: 1px solid rgba(255,255,255,0.08);
        color: ${VQ.white};
        padding: 16px 24px;
        border-radius: ${VQ.radiusMd};
        font-family: ${VQ.font};
        font-size: 14px;
        box-shadow: ${VQ.shadowLg};
        z-index: 999999;
        cursor: pointer;
        display: flex;
        align-items: center;
        gap: 12px;
        transition: transform 0.2s ease, box-shadow 0.2s ease;
      ">
        <div style="
          width: 36px; height: 36px;
          background: rgba(184,241,42,0.12);
          border-radius: 8px;
          display: flex; align-items: center; justify-content: center;
          font-size: 18px;
          flex-shrink: 0;
        ">🛡️</div>
        <div>
          <div style="font-weight: 600; font-size: 14px; margin-bottom: 2px;">
            Veriqid <span style="color: ${VQ.accent};">detected</span>
          </div>
          <div id="veriqid-badge-text" style="font-size: 12px; color: ${VQ.gray300};">
            Click to auto-fill ${formInfo.type} form
          </div>
        </div>
      </div>
    `;
    document.body.appendChild(badge);

    const inner = document.getElementById("veriqid-badge-inner");
    inner.addEventListener("mouseenter", () => {
      inner.style.transform = "translateY(-2px)";
      inner.style.boxShadow = "0 12px 40px rgba(0,0,0,0.3)";
    });
    inner.addEventListener("mouseleave", () => {
      inner.style.transform = "translateY(0)";
      inner.style.boxShadow = VQ.shadowLg;
    });
    inner.addEventListener("click", () => handleAutoFill(formInfo));
  }

  function updateBadge(status, message) {
    const text = document.getElementById("veriqid-badge-text");
    const inner = document.getElementById("veriqid-badge-inner");
    if (!text || !inner) return;

    text.textContent = message;

    if (status === "loading") {
      inner.style.borderColor = VQ.warning;
      text.style.color = VQ.warning;
    } else if (status === "success") {
      inner.style.borderColor = VQ.success;
      text.style.color = VQ.success;
    } else if (status === "error") {
      inner.style.borderColor = VQ.error;
      text.style.color = VQ.error;
    }
  }

  // ---- Auto-Fill Logic ----

  async function handleAutoFill(formInfo) {
    try {
      updateBadge("loading", "Generating proof...");

      if (formInfo.type === "signup") {
        await handleSignupAutoFill(formInfo);
      } else if (formInfo.type === "login") {
        await handleLoginAutoFill(formInfo);
      }

      updateBadge("success", "Auto-filled successfully!");

      // Check auto-submit
      const config = await chrome.runtime.sendMessage({ action: "getConfig" });
      if (config.autoSubmit) {
        setTimeout(() => {
          const form = findParentForm(formInfo);
          if (form) form.submit();
        }, 500);
      }

      // Fade out after 3s
      setTimeout(() => {
        const badge = document.getElementById("veriqid-badge");
        if (badge) {
          badge.style.transition = "opacity 0.5s ease";
          badge.style.opacity = "0";
          setTimeout(() => badge.remove(), 500);
        }
      }, 3000);
    } catch (error) {
      console.error("Veriqid auto-fill error:", error);
      updateBadge("error", error.message);
    }
  }

  async function handleSignupAutoFill(formInfo) {
    const response = await chrome.runtime.sendMessage({
      action: "generateRegistrationProof",
      challenge: formInfo.challenge,
      serviceName: formInfo.serviceName,
    });
    if (!response.success && !response.proof_hex) {
      throw new Error(response.error || "Failed to generate registration proof");
    }
    setFieldValue(formInfo.fields.spk, response.spk_hex);
    setFieldValue(formInfo.fields.proof, response.proof_hex);
    setFieldValue(formInfo.fields.ringSize, String(response.ring_size));
  }

  async function handleLoginAutoFill(formInfo) {
    const response = await chrome.runtime.sendMessage({
      action: "generateAuthProof",
      challenge: formInfo.challenge,
      serviceName: formInfo.serviceName,
    });
    if (!response.success && !response.auth_proof_hex) {
      throw new Error(response.error || "Failed to generate auth proof");
    }
    setFieldValue(formInfo.fields.authProof, response.auth_proof_hex);
    if (response.spk_hex && formInfo.fields.spk) {
      setFieldValue(formInfo.fields.spk, response.spk_hex);
    }
  }

  // ---- Helpers ----

  function setFieldValue(element, value) {
    if (!element || !value) return;
    element.value = value;
    // Trigger events for React/Vue/Angular compatibility
    element.dispatchEvent(new Event("input", { bubbles: true }));
    element.dispatchEvent(new Event("change", { bubbles: true }));
    // Lime highlight flash
    const orig = element.style.backgroundColor;
    const origBorder = element.style.borderColor;
    element.style.backgroundColor = "rgba(184,241,42,0.1)";
    element.style.borderColor = "#B8F12A";
    element.style.transition = "background-color 0.5s ease, border-color 0.5s ease";
    setTimeout(() => {
      element.style.backgroundColor = orig;
      element.style.borderColor = origBorder;
    }, 1500);
  }

  function findParentForm(formInfo) {
    for (const field of Object.values(formInfo.fields)) {
      if (field && field.closest) {
        const form = field.closest("form");
        if (form) return form;
      }
    }
    return null;
  }

  // ---- Main ----

  function init() {
    const formInfo = detectVeriqidForm();
    if (!formInfo) return;

    console.log(`[Veriqid] Detected ${formInfo.type} form`);

    chrome.runtime.sendMessage({ action: "pageHasVeriqidForm", formType: formInfo.type });
    injectVeriqidBadge(formInfo);

    chrome.runtime.sendMessage({ action: "getConfig" }, (config) => {
      if (config && config.autoFill) handleAutoFill(formInfo);
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }

  // MutationObserver for SPAs
  const observer = new MutationObserver(() => {
    if (document.getElementById("veriqid-badge")) return;
    const formInfo = detectVeriqidForm();
    if (formInfo) {
      observer.disconnect();
      chrome.runtime.sendMessage({ action: "pageHasVeriqidForm", formType: formInfo.type });
      injectVeriqidBadge(formInfo);
    }
  });
  observer.observe(document.body || document.documentElement, { childList: true, subtree: true });
  setTimeout(() => observer.disconnect(), 10000);
})();
```

---

## STEP 7: Write the Popup UI (popup.html + popup.css + popup.js)

**Time: ~1 hour**

The popup uses the full Veriqid design system — dark teal background, lime accents, clean typography.

### 7.1 Create `extension/popup.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Veriqid</title>
  <link rel="stylesheet" href="popup.css">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
</head>
<body>
  <div class="container">
    <!-- Header -->
    <header class="header">
      <div class="header-left">
        <div class="logo-icon">🛡️</div>
        <h1>Veriqid</h1>
      </div>
      <div class="status-indicator" id="status-indicator">
        <span class="status-dot" id="status-dot"></span>
      </div>
    </header>

    <!-- ======== Setup Screen ======== -->
    <div id="screen-setup" class="screen" style="display: none;">
      <div class="screen-intro">
        <h2>Welcome to <span class="accent">Veriqid</span></h2>
        <p>Privacy-first identity for children. Let's get set up.</p>
      </div>

      <div class="form-group">
        <label for="setup-bridge-url">Bridge URL</label>
        <input type="text" id="setup-bridge-url" class="input" value="http://127.0.0.1:9090" placeholder="http://127.0.0.1:9090">
        <span class="help-text">Local bridge server address</span>
      </div>

      <div class="form-group">
        <label for="setup-keypath">Identity Key Path</label>
        <input type="text" id="setup-keypath" class="input" placeholder="./key1" value="./key1">
        <span class="help-text">Path to master secret key (relative to bridge directory)</span>
      </div>

      <div class="form-group">
        <label for="setup-ethkey">Ethereum Private Key</label>
        <input type="password" id="setup-ethkey" class="input" placeholder="Ganache key without 0x prefix">
        <span class="help-text">For on-chain registration (test key only)</span>
      </div>

      <button id="btn-setup-save" class="btn-primary">Save & Connect</button>
      <div id="setup-error" class="error-box" style="display: none;"></div>
    </div>

    <!-- ======== Main Screen ======== -->
    <div id="screen-main" class="screen" style="display: none;">

      <!-- Bridge Status Card -->
      <div class="card">
        <div class="card-header">
          <span>Bridge Status</span>
          <button id="btn-refresh" class="btn-icon" title="Refresh">↻</button>
        </div>
        <div class="card-body">
          <div class="stat-row">
            <span class="stat-label">Status</span>
            <span id="bridge-status" class="stat-value">Checking...</span>
          </div>
          <div class="stat-row">
            <span class="stat-label">Contract</span>
            <span id="bridge-contract" class="stat-value mono">—</span>
          </div>
          <div class="stat-row">
            <span class="stat-label">Version</span>
            <span id="bridge-version" class="stat-value">—</span>
          </div>
        </div>
      </div>

      <!-- Active Identity Card -->
      <div class="card">
        <div class="card-header">
          <span>Active Identity</span>
        </div>
        <div class="card-body">
          <div class="stat-row">
            <span class="stat-label">Key</span>
            <span id="active-keypath" class="stat-value mono">—</span>
          </div>
          <div style="margin-top: 10px;">
            <button id="btn-create-identity" class="btn-secondary btn-sm">+ New Identity</button>
          </div>
        </div>
      </div>

      <!-- Settings Card -->
      <div class="card">
        <div class="card-header">
          <span>Settings</span>
        </div>
        <div class="card-body">
          <div class="toggle-row">
            <label for="toggle-autofill">Auto-fill forms</label>
            <input type="checkbox" id="toggle-autofill" class="toggle" checked>
          </div>
          <div class="toggle-row">
            <label for="toggle-autosubmit">Auto-submit after fill</label>
            <input type="checkbox" id="toggle-autosubmit" class="toggle">
          </div>
        </div>
      </div>

      <div class="footer-actions">
        <button id="btn-settings" class="btn-link">Change Bridge Settings</button>
      </div>
    </div>

    <!-- ======== Create Identity Screen ======== -->
    <div id="screen-create" class="screen" style="display: none;">
      <div class="screen-intro">
        <h2>Create <span class="accent">Identity</span></h2>
        <p>Generate a new master key and register on-chain.</p>
      </div>

      <div class="form-group">
        <label for="create-keypath">Key File Path</label>
        <input type="text" id="create-keypath" class="input" placeholder="./key3">
      </div>

      <div class="btn-row">
        <button id="btn-create-confirm" class="btn-primary">Create</button>
        <button id="btn-create-cancel" class="btn-secondary">Cancel</button>
      </div>

      <div id="create-result" class="result-box" style="display: none;">
        <div class="stat-row">
          <span class="stat-label">MPK</span>
          <span id="create-mpk" class="stat-value mono"></span>
        </div>
        <div class="stat-row">
          <span class="stat-label">Index</span>
          <span id="create-index" class="stat-value"></span>
        </div>
      </div>

      <div id="create-error" class="error-box" style="display: none;"></div>
    </div>
  </div>

  <script src="popup.js"></script>
</body>
</html>
```

### 7.2 Create `extension/popup.css`

```css
/* ============================================================
   Veriqid Extension Popup — Design System
   ============================================================ */

:root {
  --vq-dark:         #0A1F2F;
  --vq-dark-mid:     #0F2B3C;
  --vq-dark-light:   #153A4F;
  --vq-accent:       #B8F12A;
  --vq-accent-hover: #A3D925;
  --vq-white:        #FFFFFF;
  --vq-gray-100:     #E2E8F0;
  --vq-gray-200:     #CBD5E1;
  --vq-gray-300:     #94A3B8;
  --vq-gray-400:     #64748B;
  --vq-success:      #4ADE80;
  --vq-error:        #F87171;
  --vq-warning:      #FBBF24;
  --vq-radius-sm:    8px;
  --vq-radius-md:    12px;
  --vq-radius-full:  9999px;
  --vq-font:         'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  --vq-mono:         'SF Mono', 'Fira Code', 'Consolas', monospace;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  width: 380px;
  min-height: 240px;
  font-family: var(--vq-font);
  font-size: 13px;
  color: var(--vq-gray-200);
  background: var(--vq-dark);
}

/* ---- Header ---- */

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  width: 32px; height: 32px;
  background: rgba(184,241,42,0.12);
  border-radius: var(--vq-radius-sm);
  display: flex; align-items: center; justify-content: center;
  font-size: 18px;
}

.header h1 {
  font-size: 17px;
  font-weight: 700;
  color: var(--vq-white);
  letter-spacing: -0.3px;
}

.status-indicator { display: flex; align-items: center; }

.status-dot {
  width: 10px; height: 10px;
  border-radius: 50%;
  background: var(--vq-gray-400);
  transition: background 0.3s;
}
.status-dot.connected { background: var(--vq-success); }
.status-dot.disconnected { background: var(--vq-error); }

/* ---- Screens ---- */

.screen { padding: 16px 18px; }

.screen-intro { margin-bottom: 18px; }
.screen-intro h2 {
  font-size: 17px;
  font-weight: 700;
  color: var(--vq-white);
  margin-bottom: 4px;
}
.screen-intro p {
  font-size: 12px;
  color: var(--vq-gray-300);
}

.accent { color: var(--vq-accent); }

/* ---- Cards ---- */

.card {
  background: var(--vq-dark-mid);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: var(--vq-radius-md);
  margin-bottom: 12px;
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  font-weight: 600;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--vq-gray-300);
  border-bottom: 1px solid rgba(255,255,255,0.04);
}

.card-body { padding: 12px 14px; }

.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 0;
}
.stat-label { color: var(--vq-gray-400); font-size: 12px; }
.stat-value { color: var(--vq-white); font-size: 12px; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mono { font-family: var(--vq-mono); font-size: 11px; }

/* ---- Forms ---- */

.form-group { margin-bottom: 14px; }
.form-group label {
  display: block;
  font-weight: 600;
  font-size: 12px;
  color: var(--vq-gray-300);
  margin-bottom: 5px;
}
.input {
  width: 100%;
  padding: 10px 14px;
  background: var(--vq-dark-light);
  border: 1.5px solid rgba(255,255,255,0.1);
  border-radius: var(--vq-radius-sm);
  color: var(--vq-white);
  font-size: 13px;
  font-family: var(--vq-font);
  transition: border-color 0.2s, box-shadow 0.2s;
}
.input::placeholder { color: var(--vq-gray-400); }
.input:focus {
  outline: none;
  border-color: var(--vq-accent);
  box-shadow: 0 0 0 3px rgba(184,241,42,0.12);
}
.help-text {
  display: block;
  font-size: 11px;
  color: var(--vq-gray-400);
  margin-top: 3px;
}

/* ---- Buttons ---- */

.btn-primary {
  display: block; width: 100%;
  padding: 11px 24px;
  background: var(--vq-accent);
  color: var(--vq-dark);
  font-weight: 600; font-size: 14px;
  border: none;
  border-radius: var(--vq-radius-full);
  cursor: pointer;
  font-family: var(--vq-font);
  transition: background 0.2s, transform 0.1s;
}
.btn-primary:hover { background: var(--vq-accent-hover); transform: translateY(-1px); }
.btn-primary:disabled { background: var(--vq-gray-400); cursor: not-allowed; transform: none; }

.btn-secondary {
  display: inline-block;
  padding: 8px 18px;
  background: transparent;
  color: var(--vq-gray-200);
  font-weight: 500; font-size: 13px;
  border: 1.5px solid rgba(255,255,255,0.15);
  border-radius: var(--vq-radius-full);
  cursor: pointer;
  font-family: var(--vq-font);
  transition: border-color 0.2s, color 0.2s;
}
.btn-secondary:hover { border-color: var(--vq-accent); color: var(--vq-accent); }

.btn-sm { padding: 6px 14px; font-size: 12px; }

.btn-icon {
  background: none; border: none;
  cursor: pointer; font-size: 16px;
  color: var(--vq-gray-300);
  padding: 2px 6px; border-radius: 4px;
}
.btn-icon:hover { background: rgba(255,255,255,0.06); color: var(--vq-white); }

.btn-link {
  background: none; border: none;
  color: var(--vq-gray-300);
  font-size: 12px; cursor: pointer;
  font-family: var(--vq-font);
  transition: color 0.2s;
}
.btn-link:hover { color: var(--vq-accent); }

.btn-row { display: flex; gap: 10px; margin-top: 8px; }
.btn-row .btn-primary { flex: 1; }
.btn-row .btn-secondary { flex: 1; text-align: center; }

/* ---- Toggles ---- */

.toggle-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
}
.toggle-row label { font-size: 13px; color: var(--vq-gray-200); }
.toggle {
  width: 18px; height: 18px;
  accent-color: var(--vq-accent);
}

/* ---- Feedback ---- */

.error-box {
  font-size: 12px; margin-top: 10px;
  padding: 10px 14px;
  background: rgba(248,113,113,0.1);
  border: 1px solid rgba(248,113,113,0.3);
  border-radius: var(--vq-radius-sm);
  color: var(--vq-error);
}

.result-box {
  margin-top: 12px;
  padding: 12px 14px;
  background: rgba(184,241,42,0.06);
  border: 1px solid rgba(184,241,42,0.15);
  border-radius: var(--vq-radius-sm);
}

.footer-actions { text-align: center; padding-top: 6px; }
```

### 7.3 Create `extension/popup.js`

```javascript
// ============================================================
// Veriqid Extension Popup Logic
// ============================================================

document.addEventListener("DOMContentLoaded", async () => {
  // ---- References ----
  const screenSetup  = document.getElementById("screen-setup");
  const screenMain   = document.getElementById("screen-main");
  const screenCreate = document.getElementById("screen-create");
  const statusDot    = document.getElementById("status-dot");

  const setupBridgeUrl = document.getElementById("setup-bridge-url");
  const setupKeypath   = document.getElementById("setup-keypath");
  const setupEthkey    = document.getElementById("setup-ethkey");
  const btnSetupSave   = document.getElementById("btn-setup-save");
  const setupError     = document.getElementById("setup-error");

  const bridgeStatus   = document.getElementById("bridge-status");
  const bridgeContract = document.getElementById("bridge-contract");
  const bridgeVersion  = document.getElementById("bridge-version");
  const activeKeypath  = document.getElementById("active-keypath");
  const btnRefresh     = document.getElementById("btn-refresh");
  const btnCreateId    = document.getElementById("btn-create-identity");
  const btnSettings    = document.getElementById("btn-settings");
  const toggleAutofill   = document.getElementById("toggle-autofill");
  const toggleAutosubmit = document.getElementById("toggle-autosubmit");

  const createKeypath     = document.getElementById("create-keypath");
  const btnCreateConfirm  = document.getElementById("btn-create-confirm");
  const btnCreateCancel   = document.getElementById("btn-create-cancel");
  const createResult      = document.getElementById("create-result");
  const createMpk         = document.getElementById("create-mpk");
  const createIndex       = document.getElementById("create-index");
  const createError       = document.getElementById("create-error");

  // ---- Helpers ----
  function showScreen(s) {
    screenSetup.style.display = "none";
    screenMain.style.display = "none";
    screenCreate.style.display = "none";
    s.style.display = "block";
  }
  function showError(el, msg) { el.textContent = msg; el.style.display = "block"; }
  function hideError(el) { el.style.display = "none"; }
  function trunc(s, start, end) {
    if (!s || s.length < start + end + 3) return s || "—";
    return s.slice(0, start) + "..." + s.slice(-end);
  }

  // ---- Initialize ----
  const config = await chrome.runtime.sendMessage({ action: "getConfig" });

  if (!config.isSetup) {
    showScreen(screenSetup);
    if (config.bridgeUrl) setupBridgeUrl.value = config.bridgeUrl;
    if (config.keypath) setupKeypath.value = config.keypath;
  } else {
    showScreen(screenMain);
    toggleAutofill.checked = config.autoFill;
    toggleAutosubmit.checked = config.autoSubmit;
    activeKeypath.textContent = config.keypath || "Not set";
    await refreshBridge();
  }

  // ---- Setup ----
  btnSetupSave.addEventListener("click", async () => {
    const bridgeUrl = setupBridgeUrl.value.trim();
    const keypath = setupKeypath.value.trim();
    const ethkey = setupEthkey.value.trim();
    if (!bridgeUrl || !keypath) { showError(setupError, "Bridge URL and Key Path are required."); return; }

    btnSetupSave.disabled = true;
    btnSetupSave.textContent = "Connecting...";
    hideError(setupError);

    try {
      await chrome.runtime.sendMessage({
        action: "saveConfig",
        config: { bridgeUrl, keypath, ethkey, isSetup: true },
      });

      const status = await chrome.runtime.sendMessage({ action: "checkBridge" });
      if (!status.connected) {
        showError(setupError, "Could not connect to bridge at " + bridgeUrl + ". Is it running?");
        btnSetupSave.disabled = false;
        btnSetupSave.textContent = "Save & Connect";
        return;
      }

      activeKeypath.textContent = keypath;
      toggleAutofill.checked = true;
      toggleAutosubmit.checked = false;
      showScreen(screenMain);
      await refreshBridge();
    } catch (e) {
      showError(setupError, e.message);
      btnSetupSave.disabled = false;
      btnSetupSave.textContent = "Save & Connect";
    }
  });

  // ---- Main ----
  async function refreshBridge() {
    bridgeStatus.textContent = "Checking...";
    bridgeStatus.style.color = "";
    const s = await chrome.runtime.sendMessage({ action: "checkBridge" });
    if (s.connected) {
      statusDot.className = "status-dot connected";
      bridgeStatus.textContent = "Connected";
      bridgeStatus.style.color = "#4ADE80";
      bridgeContract.textContent = trunc(s.contract, 8, 6);
      bridgeContract.title = s.contract || "";
      bridgeVersion.textContent = s.version || "—";
    } else {
      statusDot.className = "status-dot disconnected";
      bridgeStatus.textContent = "Disconnected";
      bridgeStatus.style.color = "#F87171";
      bridgeContract.textContent = "—";
      bridgeVersion.textContent = "—";
    }
  }

  btnRefresh.addEventListener("click", refreshBridge);
  btnCreateId.addEventListener("click", () => {
    showScreen(screenCreate);
    createResult.style.display = "none";
    hideError(createError);
    createKeypath.value = "";
  });
  btnSettings.addEventListener("click", () => {
    showScreen(screenSetup);
    chrome.runtime.sendMessage({ action: "getConfig" }).then(cfg => {
      setupBridgeUrl.value = cfg.bridgeUrl;
      setupKeypath.value = cfg.keypath;
      setupEthkey.value = cfg.ethkey || "";
      btnSetupSave.disabled = false;
      btnSetupSave.textContent = "Save & Connect";
    });
  });

  toggleAutofill.addEventListener("change", () => {
    chrome.runtime.sendMessage({ action: "saveConfig", config: { autoFill: toggleAutofill.checked } });
  });
  toggleAutosubmit.addEventListener("change", () => {
    chrome.runtime.sendMessage({ action: "saveConfig", config: { autoSubmit: toggleAutosubmit.checked } });
  });

  // ---- Create Identity ----
  btnCreateConfirm.addEventListener("click", async () => {
    const keypath = createKeypath.value.trim();
    if (!keypath) { showError(createError, "Key path is required (e.g., ./key3)"); return; }

    btnCreateConfirm.disabled = true;
    btnCreateConfirm.textContent = "Creating...";
    hideError(createError);
    createResult.style.display = "none";

    try {
      const config = await chrome.runtime.sendMessage({ action: "getConfig" });
      if (!config.ethkey) {
        showError(createError, "Ethereum key not configured. Go to settings first.");
        return;
      }
      const result = await chrome.runtime.sendMessage({
        action: "createIdentity", keypath, ethkey: config.ethkey,
      });
      if (!result.success) throw new Error(result.error || "Creation failed");

      createMpk.textContent = trunc(result.mpk_hex, 12, 8);
      createMpk.title = result.mpk_hex;
      createIndex.textContent = String(result.index);
      createResult.style.display = "block";

      await chrome.runtime.sendMessage({ action: "saveConfig", config: { keypath } });
      activeKeypath.textContent = keypath;
    } catch (e) {
      showError(createError, e.message);
    } finally {
      btnCreateConfirm.disabled = false;
      btnCreateConfirm.textContent = "Create";
    }
  });

  btnCreateCancel.addEventListener("click", () => showScreen(screenMain));
});
```

---

## STEP 8: Redesign the Phase 4 Server Templates

**Time: ~1 hour**

Phase 4's templates were functional but unstyled. Now we redesign ALL of them using the Veriqid design system — dark teal backgrounds, lime accents, pill-shaped buttons, stat badges, and the same visual language as the reference design.

### 8.1 Create `templates/base.css` (shared styles)

Save this file at `Veriqid/templates/base.css` — it will be linked from all templates.

> **IMPORTANT:** The Veriqid server serves static files from `./static/`. So place this file at `Veriqid/static/base.css` and reference it as `/static/base.css` in templates.

```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
```

Create **`static/base.css`**:

```css
/* ============================================================
   Veriqid Design System — Server Page Styles
   ============================================================ */

@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

:root {
  --vq-dark:         #0A1F2F;
  --vq-dark-mid:     #0F2B3C;
  --vq-dark-light:   #153A4F;
  --vq-accent:       #B8F12A;
  --vq-accent-hover: #A3D925;
  --vq-white:        #FFFFFF;
  --vq-off-white:    #F0F4F8;
  --vq-gray-100:     #E2E8F0;
  --vq-gray-200:     #CBD5E1;
  --vq-gray-300:     #94A3B8;
  --vq-gray-400:     #64748B;
  --vq-gray-500:     #475569;
  --vq-success:      #4ADE80;
  --vq-error:        #F87171;
  --vq-warning:      #FBBF24;
  --vq-info:         #60A5FA;
  --vq-shadow-sm:    0 1px 3px rgba(0,0,0,0.12);
  --vq-shadow-md:    0 4px 12px rgba(0,0,0,0.15);
  --vq-shadow-lg:    0 8px 30px rgba(0,0,0,0.2);
  --vq-shadow-card:  0 4px 20px rgba(10,31,47,0.3);
  --vq-radius-sm:    8px;
  --vq-radius-md:    12px;
  --vq-radius-lg:    16px;
  --vq-radius-xl:    24px;
  --vq-radius-full:  9999px;
  --vq-font:         'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  --vq-mono:         'SF Mono', 'Fira Code', 'Consolas', monospace;
}

*, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: var(--vq-font);
  background: var(--vq-dark);
  color: var(--vq-gray-200);
  min-height: 100vh;
  -webkit-font-smoothing: antialiased;
}

/* ---- Navigation ---- */

.vq-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 48px;
  background: transparent;
}

.vq-nav-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
}

.vq-nav-brand-icon {
  width: 36px; height: 36px;
  background: rgba(184,241,42,0.12);
  border-radius: var(--vq-radius-sm);
  display: flex; align-items: center; justify-content: center;
  font-size: 20px;
}

.vq-nav-brand-name {
  font-size: 20px;
  font-weight: 700;
  color: var(--vq-white);
  letter-spacing: -0.3px;
}

.vq-nav-links {
  display: flex;
  gap: 32px;
  list-style: none;
}

.vq-nav-link {
  color: var(--vq-gray-200);
  text-decoration: none;
  font-size: 15px;
  font-weight: 500;
  padding-bottom: 4px;
  border-bottom: 2px solid transparent;
  transition: color 0.2s, border-color 0.2s;
}

.vq-nav-link:hover { color: var(--vq-white); }
.vq-nav-link.active { color: var(--vq-white); border-bottom-color: var(--vq-white); }

.vq-nav-action .vq-btn-outline {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 22px;
  border: 1.5px solid rgba(255,255,255,0.2);
  border-radius: var(--vq-radius-full);
  color: var(--vq-white);
  text-decoration: none;
  font-size: 14px; font-weight: 500;
  transition: border-color 0.2s, background 0.2s;
}
.vq-nav-action .vq-btn-outline:hover {
  border-color: var(--vq-accent);
  background: rgba(184,241,42,0.06);
}

/* ---- Hero Section ---- */

.vq-hero {
  padding: 60px 48px 80px;
  position: relative;
  overflow: hidden;
}

.vq-hero-subtitle {
  font-size: 14px;
  color: var(--vq-gray-300);
  margin-bottom: 16px;
  letter-spacing: 0.5px;
}

.vq-hero-title {
  font-size: 52px;
  font-weight: 700;
  color: var(--vq-white);
  line-height: 1.1;
  max-width: 600px;
  margin-bottom: 20px;
}

.vq-hero-title .accent { color: var(--vq-accent); }

.vq-hero-desc {
  font-size: 16px;
  color: var(--vq-gray-300);
  max-width: 480px;
  line-height: 1.6;
  margin-bottom: 32px;
}

/* ---- Buttons ---- */

.vq-btn-primary {
  display: inline-block;
  padding: 14px 32px;
  background: var(--vq-accent);
  color: var(--vq-dark);
  font-weight: 600; font-size: 15px;
  border: none;
  border-radius: var(--vq-radius-full);
  cursor: pointer;
  text-decoration: none;
  font-family: var(--vq-font);
  transition: background 0.2s, transform 0.1s;
}
.vq-btn-primary:hover { background: var(--vq-accent-hover); transform: translateY(-1px); }

.vq-btn-secondary {
  display: inline-block;
  padding: 12px 28px;
  background: transparent;
  color: var(--vq-white);
  font-weight: 500; font-size: 14px;
  border: 1.5px solid var(--vq-gray-300);
  border-radius: var(--vq-radius-full);
  cursor: pointer;
  text-decoration: none;
  font-family: var(--vq-font);
  transition: border-color 0.2s, color 0.2s;
}
.vq-btn-secondary:hover { border-color: var(--vq-accent); color: var(--vq-accent); }

/* ---- Stat Badges ---- */

.vq-stat-badges {
  display: flex; gap: 16px; flex-wrap: wrap;
  margin-top: 40px;
}

.vq-stat-badge {
  background: var(--vq-dark-mid);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: var(--vq-radius-md);
  padding: 16px 22px;
  min-width: 140px;
  box-shadow: var(--vq-shadow-card);
}

.vq-stat-badge-icon {
  font-size: 20px;
  margin-bottom: 8px;
  color: var(--vq-accent);
}

.vq-stat-badge-label {
  font-size: 12px;
  color: var(--vq-gray-300);
  margin-bottom: 2px;
}

.vq-stat-badge-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--vq-accent);
}

/* ---- Feature Cards (white section) ---- */

.vq-features {
  background: var(--vq-off-white);
  padding: 60px 48px;
}

.vq-features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 20px;
}

.vq-feature-card {
  background: var(--vq-white);
  border-radius: var(--vq-radius-lg);
  padding: 28px 24px;
  box-shadow: var(--vq-shadow-sm);
  transition: box-shadow 0.2s, transform 0.2s;
}
.vq-feature-card:hover { box-shadow: var(--vq-shadow-md); transform: translateY(-2px); }
.vq-feature-card.highlighted { background: var(--vq-accent); color: var(--vq-dark); }
.vq-feature-card.highlighted .vq-feature-icon { background: rgba(10,31,47,0.1); }
.vq-feature-card.highlighted .vq-feature-desc { color: var(--vq-dark-light); }

.vq-feature-icon {
  width: 48px; height: 48px;
  background: rgba(184,241,42,0.1);
  border-radius: var(--vq-radius-sm);
  display: flex; align-items: center; justify-content: center;
  font-size: 22px;
  margin-bottom: 16px;
}

.vq-feature-title {
  font-size: 16px; font-weight: 600;
  color: var(--vq-dark);
  margin-bottom: 8px;
}

.vq-feature-desc {
  font-size: 13px;
  color: var(--vq-gray-500);
  line-height: 1.5;
}

/* ---- Form Page Layout ---- */

.vq-form-page {
  display: flex;
  min-height: 100vh;
}

.vq-form-sidebar {
  flex: 0 0 45%;
  background: var(--vq-dark);
  padding: 60px 48px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.vq-form-main {
  flex: 1;
  background: var(--vq-dark-mid);
  padding: 60px 48px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.vq-form-box {
  width: 100%;
  max-width: 440px;
}

.vq-form-box h2 {
  font-size: 28px;
  font-weight: 700;
  color: var(--vq-white);
  margin-bottom: 8px;
}

.vq-form-box p {
  font-size: 14px;
  color: var(--vq-gray-300);
  margin-bottom: 28px;
}

.vq-form-group {
  margin-bottom: 18px;
}

.vq-form-label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--vq-gray-300);
  margin-bottom: 6px;
}

.vq-form-input {
  width: 100%;
  padding: 12px 16px;
  background: var(--vq-dark-light);
  border: 1.5px solid rgba(255,255,255,0.1);
  border-radius: var(--vq-radius-sm);
  color: var(--vq-white);
  font-size: 14px;
  font-family: var(--vq-font);
  transition: border-color 0.2s, box-shadow 0.2s;
}
.vq-form-input::placeholder { color: var(--vq-gray-400); }
.vq-form-input:focus {
  outline: none;
  border-color: var(--vq-accent);
  box-shadow: 0 0 0 3px rgba(184,241,42,0.12);
}

.vq-form-hint {
  font-size: 11px;
  color: var(--vq-gray-400);
  margin-top: 3px;
}

.vq-form-submit {
  margin-top: 24px;
}

/* ---- Dashboard ---- */

.vq-dashboard {
  padding: 48px;
}

.vq-dashboard-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 32px;
}

.vq-dashboard-welcome {
  font-size: 28px;
  font-weight: 700;
  color: var(--vq-white);
}

.vq-dashboard-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.vq-dashboard-card {
  background: var(--vq-dark-mid);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: var(--vq-radius-lg);
  padding: 28px;
}

.vq-dashboard-card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--vq-gray-300);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 16px;
}

.vq-dashboard-card-value {
  font-size: 32px;
  font-weight: 700;
  color: var(--vq-accent);
}

.vq-dashboard-card-desc {
  font-size: 13px;
  color: var(--vq-gray-400);
  margin-top: 4px;
}

/* ---- Result Page ---- */

.vq-result-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 48px;
}

.vq-result-box {
  text-align: center;
  max-width: 480px;
}

.vq-result-icon {
  width: 80px; height: 80px;
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 40px;
  margin: 0 auto 24px;
}
.vq-result-icon.success { background: rgba(74,222,128,0.12); }
.vq-result-icon.failure { background: rgba(248,113,113,0.12); }

.vq-result-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--vq-white);
  margin-bottom: 12px;
}

.vq-result-desc {
  font-size: 15px;
  color: var(--vq-gray-300);
  margin-bottom: 32px;
  line-height: 1.5;
}

/* ---- Review Badge ---- */

.vq-review {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 32px;
}

.vq-review-avatars {
  display: flex;
}

.vq-review-avatar {
  width: 36px; height: 36px;
  border-radius: 50%;
  background: var(--vq-dark-light);
  border: 2px solid var(--vq-dark);
  margin-left: -8px;
  display: flex; align-items: center; justify-content: center;
  font-size: 14px;
}
.vq-review-avatar:first-child { margin-left: 0; }

.vq-review-score {
  width: 40px; height: 40px;
  background: var(--vq-dark-mid);
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 700;
  color: var(--vq-accent);
  margin-left: 4px;
}

.vq-review-text {
  font-size: 14px; font-weight: 600;
  color: var(--vq-white);
}

.vq-review-sub {
  font-size: 12px;
  color: var(--vq-gray-400);
}

/* ---- Responsive ---- */

@media (max-width: 768px) {
  .vq-nav { padding: 14px 20px; }
  .vq-hero { padding: 40px 20px 60px; }
  .vq-hero-title { font-size: 36px; }
  .vq-features { padding: 40px 20px; }
  .vq-form-page { flex-direction: column; }
  .vq-form-sidebar { flex: none; padding: 40px 24px; }
  .vq-form-main { padding: 40px 24px; }
  .vq-dashboard { padding: 24px; }
}
```

### 8.2 Redesign `templates/index.html` (Home Page)

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.PlatformName}} — Powered by Veriqid</title>
  <link rel="stylesheet" href="/static/base.css">
</head>
<body>
  <!-- Nav -->
  <nav class="vq-nav">
    <a href="/" class="vq-nav-brand">
      <div class="vq-nav-brand-icon">🛡️</div>
      <span class="vq-nav-brand-name">{{.PlatformName}}</span>
    </a>
    <ul class="vq-nav-links">
      <li><a href="/" class="vq-nav-link active">Home</a></li>
      <li><a href="/signup" class="vq-nav-link">Sign Up</a></li>
      <li><a href="/login" class="vq-nav-link">Log In</a></li>
    </ul>
    <div class="vq-nav-action">
      <a href="/login" class="vq-btn-outline">👤 Log In</a>
    </div>
  </nav>

  <!-- Hero -->
  <section class="vq-hero">
    <p class="vq-hero-subtitle">Your Privacy-First Platform</p>
    <h1 class="vq-hero-title">
      Verify Once,<br>
      Play <span class="accent">Everywhere</span>
    </h1>
    <p class="vq-hero-desc">
      Access {{.PlatformName}} with zero data exposure. Your child's identity stays private — powered by Veriqid's zero-knowledge proofs.
    </p>
    <a href="/signup" class="vq-btn-primary">Get Started Now</a>

    <div class="vq-stat-badges">
      <div class="vq-stat-badge">
        <div class="vq-stat-badge-icon">🔐</div>
        <div class="vq-stat-badge-label">Zero Knowledge</div>
        <div class="vq-stat-badge-value">100%</div>
      </div>
      <div class="vq-stat-badge">
        <div class="vq-stat-badge-icon">🛡️</div>
        <div class="vq-stat-badge-label">COPPA Compliant</div>
        <div class="vq-stat-badge-value">Yes</div>
      </div>
      <div class="vq-stat-badge">
        <div class="vq-stat-badge-icon">⚡</div>
        <div class="vq-stat-badge-label">Verification</div>
        <div class="vq-stat-badge-value">&lt;2s</div>
      </div>
    </div>

    <div class="vq-review" style="margin-top: 48px;">
      <div class="vq-review-avatars">
        <div class="vq-review-avatar">👧</div>
        <div class="vq-review-avatar">👦</div>
        <div class="vq-review-avatar">👩</div>
      </div>
      <div class="vq-review-score">4.9</div>
      <div>
        <div class="vq-review-text">Parent Approved</div>
        <div class="vq-review-sub">Privacy-first verification</div>
      </div>
    </div>
  </section>

  <!-- Features -->
  <section class="vq-features">
    <div class="vq-features-grid">
      <div class="vq-feature-card">
        <div class="vq-feature-icon">🔒</div>
        <div class="vq-feature-title">Zero Data Exposure</div>
        <div class="vq-feature-desc">Platforms learn only that your child is verified — nothing else. No names, no birthdates, no tracking.</div>
      </div>
      <div class="vq-feature-card highlighted">
        <div class="vq-feature-icon">⚡</div>
        <div class="vq-feature-title">Instant Verification</div>
        <div class="vq-feature-desc">Cryptographic proof generation in under 2 seconds. The browser extension handles everything automatically.</div>
      </div>
      <div class="vq-feature-card">
        <div class="vq-feature-icon">🌐</div>
        <div class="vq-feature-title">Cross-Platform</div>
        <div class="vq-feature-desc">One identity works across every Veriqid-enabled platform. Unlinkable pseudonyms protect privacy.</div>
      </div>
      <div class="vq-feature-card">
        <div class="vq-feature-icon">👨‍👩‍👧</div>
        <div class="vq-feature-title">Parent Control</div>
        <div class="vq-feature-desc">Parents can revoke the master identity with a single click — immediately deactivating all platform accounts.</div>
      </div>
    </div>
  </section>
</body>
</html>
```

### 8.3 Redesign `templates/signup.html` (Signup Form)

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Sign Up — {{.PlatformName}}</title>
  <link rel="stylesheet" href="/static/base.css">
</head>
<body>
  <nav class="vq-nav">
    <a href="/" class="vq-nav-brand">
      <div class="vq-nav-brand-icon">🛡️</div>
      <span class="vq-nav-brand-name">{{.PlatformName}}</span>
    </a>
    <ul class="vq-nav-links">
      <li><a href="/" class="vq-nav-link">Home</a></li>
      <li><a href="/signup" class="vq-nav-link active">Sign Up</a></li>
      <li><a href="/login" class="vq-nav-link">Log In</a></li>
    </ul>
  </nav>

  <div class="vq-form-page" style="min-height: calc(100vh - 70px);">
    <div class="vq-form-sidebar">
      <h1 class="vq-hero-title" style="font-size: 38px;">
        Join with<br><span class="accent">Zero</span> Data Exposure
      </h1>
      <p class="vq-hero-desc" style="font-size: 15px;">
        Your Veriqid browser extension will auto-fill the proof fields. Just enter a username and click Sign Up.
      </p>
      <div class="vq-stat-badges">
        <div class="vq-stat-badge">
          <div class="vq-stat-badge-icon">🔐</div>
          <div class="vq-stat-badge-label">Privacy</div>
          <div class="vq-stat-badge-value">100%</div>
        </div>
        <div class="vq-stat-badge">
          <div class="vq-stat-badge-icon">⚡</div>
          <div class="vq-stat-badge-label">Auto-Fill</div>
          <div class="vq-stat-badge-value">On</div>
        </div>
      </div>
    </div>

    <div class="vq-form-main">
      <div class="vq-form-box">
        <h2>Create <span class="accent">Account</span></h2>
        <p>Sign up with your Veriqid identity — no personal data required.</p>

        <form method="POST" action="/signup">
          <!-- Hidden fields for extension detection -->
          <input type="hidden" id="veriqid-challenge" name="challenge" value="{{.Challenge}}">
          <input type="hidden" id="veriqid-service-name" name="sname" value="{{.ServiceName}}">

          <div class="vq-form-group">
            <label class="vq-form-label" for="name">Username</label>
            <input type="text" id="name" name="name" class="vq-form-input" placeholder="Choose a username" required>
          </div>

          <div class="vq-form-group">
            <label class="vq-form-label" for="veriqid-spk">Public Key (SPK)</label>
            <input type="text" id="veriqid-spk" name="spk" class="vq-form-input" placeholder="Auto-filled by Veriqid extension" style="font-family: var(--vq-mono); font-size: 12px;">
            <span class="vq-form-hint">Filled automatically by the browser extension</span>
          </div>

          <div class="vq-form-group">
            <label class="vq-form-label" for="veriqid-proof">Membership Proof</label>
            <input type="text" id="veriqid-proof" name="proof" class="vq-form-input" placeholder="Auto-filled by Veriqid extension" style="font-family: var(--vq-mono); font-size: 12px;">
          </div>

          <div class="vq-form-group">
            <label class="vq-form-label" for="veriqid-ring-size">Ring Size</label>
            <input type="text" id="veriqid-ring-size" name="n" class="vq-form-input" placeholder="Auto-filled" style="font-family: var(--vq-mono);">
          </div>

          <input type="hidden" name="nullifier" value="">

          <div class="vq-form-submit">
            <button type="submit" class="vq-btn-primary" style="width: 100%;">Sign Up</button>
          </div>
        </form>

        <p style="text-align: center; margin-top: 20px; font-size: 13px; color: var(--vq-gray-400);">
          Already have an account? <a href="/login" style="color: var(--vq-accent); text-decoration: none;">Log In</a>
        </p>
      </div>
    </div>
  </div>
</body>
</html>
```

### 8.4 Redesign `templates/login.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Log In — {{.PlatformName}}</title>
  <link rel="stylesheet" href="/static/base.css">
</head>
<body>
  <nav class="vq-nav">
    <a href="/" class="vq-nav-brand">
      <div class="vq-nav-brand-icon">🛡️</div>
      <span class="vq-nav-brand-name">{{.PlatformName}}</span>
    </a>
    <ul class="vq-nav-links">
      <li><a href="/" class="vq-nav-link">Home</a></li>
      <li><a href="/signup" class="vq-nav-link">Sign Up</a></li>
      <li><a href="/login" class="vq-nav-link active">Log In</a></li>
    </ul>
  </nav>

  <div class="vq-form-page" style="min-height: calc(100vh - 70px);">
    <div class="vq-form-sidebar">
      <h1 class="vq-hero-title" style="font-size: 38px;">
        Welcome<br><span class="accent">Back</span>
      </h1>
      <p class="vq-hero-desc" style="font-size: 15px;">
        Prove your identity without revealing it. Your Veriqid extension auto-fills everything.
      </p>
    </div>

    <div class="vq-form-main">
      <div class="vq-form-box">
        <h2>Log <span class="accent">In</span></h2>
        <p>Authenticate with your Veriqid identity — zero-knowledge proof.</p>

        <form method="POST" action="/login">
          <input type="hidden" id="veriqid-challenge" name="challenge" value="{{.Challenge}}">
          <input type="hidden" id="veriqid-service-name" name="sname" value="{{.ServiceName}}">

          <div class="vq-form-group">
            <label class="vq-form-label" for="name">Username</label>
            <input type="text" id="name" name="name" class="vq-form-input" placeholder="Your username" required>
          </div>

          <div class="vq-form-group">
            <label class="vq-form-label" for="veriqid-spk">Public Key (SPK)</label>
            <input type="text" id="veriqid-spk" name="spk" class="vq-form-input" placeholder="Auto-filled by extension" style="font-family: var(--vq-mono); font-size: 12px;">
          </div>

          <div class="vq-form-group">
            <label class="vq-form-label" for="veriqid-auth-proof">Auth Proof</label>
            <input type="text" id="veriqid-auth-proof" name="signature" class="vq-form-input" placeholder="Auto-filled by extension" style="font-family: var(--vq-mono); font-size: 12px;">
          </div>

          <div class="vq-form-submit">
            <button type="submit" class="vq-btn-primary" style="width: 100%;">Log In</button>
          </div>
        </form>

        <p style="text-align: center; margin-top: 20px; font-size: 13px; color: var(--vq-gray-400);">
          Don't have an account? <a href="/signup" style="color: var(--vq-accent); text-decoration: none;">Sign Up</a>
        </p>
      </div>
    </div>
  </div>
</body>
</html>
```

### 8.5 Redesign `templates/dashboard.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Dashboard — {{.PlatformName}}</title>
  <link rel="stylesheet" href="/static/base.css">
</head>
<body>
  <nav class="vq-nav">
    <a href="/" class="vq-nav-brand">
      <div class="vq-nav-brand-icon">🛡️</div>
      <span class="vq-nav-brand-name">{{.PlatformName}}</span>
    </a>
    <ul class="vq-nav-links">
      <li><a href="/dashboard" class="vq-nav-link active">Dashboard</a></li>
    </ul>
    <div class="vq-nav-action">
      <a href="/logout" class="vq-btn-outline">Logout</a>
    </div>
  </nav>

  <div class="vq-dashboard">
    <div class="vq-dashboard-header">
      <h1 class="vq-dashboard-welcome">
        Welcome back, <span class="accent">{{.Username}}</span>
      </h1>
    </div>

    <div class="vq-dashboard-grid">
      <div class="vq-dashboard-card">
        <div class="vq-dashboard-card-title">Account Status</div>
        <div class="vq-dashboard-card-value" style="color: var(--vq-success);">Active</div>
        <div class="vq-dashboard-card-desc">Your Veriqid identity is verified and active</div>
      </div>

      <div class="vq-dashboard-card">
        <div class="vq-dashboard-card-title">Age Bracket</div>
        <div class="vq-dashboard-card-value">{{.AgeBracketLabel}}</div>
        <div class="vq-dashboard-card-desc">Verified by trusted authority</div>
      </div>

      <div class="vq-dashboard-card">
        <div class="vq-dashboard-card-title">Member Since</div>
        <div class="vq-dashboard-card-value" style="font-size: 20px;">{{.CreatedAt}}</div>
        <div class="vq-dashboard-card-desc">Account creation date</div>
      </div>

      <div class="vq-dashboard-card">
        <div class="vq-dashboard-card-title">Last Login</div>
        <div class="vq-dashboard-card-value" style="font-size: 20px;">{{.LastLogin}}</div>
        <div class="vq-dashboard-card-desc">Most recent authentication</div>
      </div>
    </div>

    <div style="margin-top: 40px;">
      <div class="vq-dashboard-card" style="max-width: 600px;">
        <div class="vq-dashboard-card-title">Privacy Guarantee</div>
        <p style="color: var(--vq-gray-300); line-height: 1.6; font-size: 14px;">
          {{.PlatformName}} has <span class="accent" style="font-weight: 600;">zero knowledge</span> of your real identity.
          Your account is tied only to a cryptographic pseudonym (SPK) that cannot be linked to your identity on any other platform.
          Even if {{.PlatformName}}'s database were breached, no personal information could be extracted.
        </p>
      </div>
    </div>
  </div>
</body>
</html>
```

### 8.6 Redesign `templates/result.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}} — {{.PlatformName}}</title>
  <link rel="stylesheet" href="/static/base.css">
</head>
<body>
  <nav class="vq-nav">
    <a href="/" class="vq-nav-brand">
      <div class="vq-nav-brand-icon">🛡️</div>
      <span class="vq-nav-brand-name">{{.PlatformName}}</span>
    </a>
  </nav>

  <div class="vq-result-page">
    <div class="vq-result-box">
      {{if .Success}}
      <div class="vq-result-icon success">✓</div>
      <h2 class="vq-result-title">{{.Title}}</h2>
      <p class="vq-result-desc">{{.Message}}</p>
      <a href="/dashboard" class="vq-btn-primary">Go to Dashboard</a>
      {{else}}
      <div class="vq-result-icon failure">✗</div>
      <h2 class="vq-result-title">{{.Title}}</h2>
      <p class="vq-result-desc">{{.Message}}</p>
      <a href="/" class="vq-btn-secondary">Back to Home</a>
      {{end}}
    </div>
  </div>
</body>
</html>
```

---

## STEP 9: Load and Test the Extension

**Time: ~30 minutes**

### 9.1 Make sure `static/base.css` exists

```bash
# Verify the CSS file is in the right place
ls -la /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/static/base.css
```

If you haven't created it yet, copy the full CSS from Step 8.1 into `static/base.css`.

### 9.2 Load the extension in Chrome

1. Open `chrome://extensions/`
2. Enable **"Developer mode"** (top-right toggle)
3. Click **"Load unpacked"**
4. Select `C:\Users\patmekury\Downloads\Shape_Rotor\U2SSO\Veriqid\extension`
5. Verify "Veriqid" appears with no errors
6. Pin it to the toolbar (click puzzle icon → pin)

### 9.3 Start all services

**Terminal 1 — Ganache:**
```bash
ganache --port 7545
```

**Terminal 2 — Deploy + Bridge:**
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid/contracts
truffle migrate --reset --network development
# Copy contract address

cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid

go run ./cmd/client -contract 0x<ADDR> -ethkey <KEY> -command create -keypath ./key1
go run ./cmd/client -contract 0x<ADDR> -ethkey <KEY> -command create -keypath ./key2

go build -o bridge-server ./cmd/bridge/
./bridge-server -contract 0x<ADDR> -client http://127.0.0.1:7545
```

**Terminal 3 — Veriqid server:**
```bash
cd /mnt/c/Users/patmekury/Downloads/Shape_Rotor/U2SSO/Veriqid
./veriqid-server -contract 0x<ADDR> -service "KidsTube" -port 8080
```

### 9.4 Test the extension popup

1. Click the Veriqid icon → setup screen appears (dark teal design)
2. Enter bridge URL `http://127.0.0.1:9090`, keypath `./key1`, Ganache private key
3. Click "Save & Connect" → status dot turns green
4. Main screen shows "Connected"

### 9.5 Test auto-fill on signup

1. Open `http://localhost:8080/signup`
2. The dark teal styled signup page appears (split layout)
3. A floating Veriqid badge appears bottom-right
4. If auto-fill is on, the SPK, proof, and ring size fields populate automatically (lime green flash)
5. Enter a username → click "Sign Up"
6. Success result page appears with Veriqid branding

### 9.6 Test auto-fill on login

1. Open `http://localhost:8080/login`
2. Badge appears, auto-fills auth proof + SPK
3. Enter username → click "Log In"
4. Dashboard appears with stats cards

---

## STEP 10: Troubleshooting

### 10.1 Common errors

| Error | Cause | Fix |
|-------|-------|-----|
| Badge says "Bridge is not running" | Bridge server not started | Start bridge on port 9090 |
| "No identity configured" | Keypath not set | Complete setup in popup |
| Content script doesn't detect form | Missing `veriqid-*` IDs in template | Verify templates have the right hidden field IDs |
| CSS not loading on server pages | `base.css` not in `static/` folder | Place `base.css` at `Veriqid/static/base.css` |
| `chrome.runtime.sendMessage` error | Service worker crashed | Check `chrome://extensions/` errors |
| CORS error calling bridge | Bridge missing CORS middleware | Verify Phase 2 bridge has CORS headers |
| Lime green flash not visible | Input field has `display:none` | Only visible inputs get the highlight |

### 10.2 Debugging

**Background service worker console:** `chrome://extensions/` → Veriqid → "Inspect views: service worker"

**Content script console:** Open page → F12 → Console (look for `[Veriqid]` logs)

**Clear extension storage:**
```javascript
// In service worker console:
chrome.storage.local.clear(() => console.log("Cleared"));
```

---

## STEP 11: Phase 5 Completion Checklist

```
[ ] extension/manifest.json created (Manifest V3)
[ ] extension/background.js handles bridge comm, storage, messages
[ ] extension/content.js detects forms, auto-fills, shows branded badge
[ ] extension/popup.html + popup.css + popup.js — dark teal design
[ ] Extension icons in extension/icons/ (16, 48, 128px)
[ ] Extension loads in Chrome without errors
[ ] Popup: setup screen connects to bridge
[ ] Popup: main screen shows status, identity, settings
[ ] Popup: create identity works
[ ] Badge: green ON when bridge connected, red OFF when not
[ ] Content: detects signup form (veriqid-challenge + veriqid-proof)
[ ] Content: detects login form (veriqid-challenge + veriqid-auth-proof)
[ ] Content: floating badge uses Veriqid design (dark teal, lime accents)
[ ] Content: auto-fill populates spk, proof, ring_size for signup
[ ] Content: auto-fill populates auth_proof, spk for login
[ ] Content: field highlight uses lime green (#B8F12A)
[ ] Content: MutationObserver handles SPA-loaded forms
[ ] Settings: auto-fill and auto-submit toggles persist
[ ] static/base.css created with full Veriqid design system
[ ] templates/index.html redesigned — hero + stat badges + feature cards
[ ] templates/signup.html redesigned — split layout + dark forms
[ ] templates/login.html redesigned — split layout + dark forms
[ ] templates/dashboard.html redesigned — stat grid + privacy card
[ ] templates/result.html redesigned — centered result with icon
[ ] End-to-end flow: extension → bridge → server → success page (all styled)
```

---

## What Each File Does (Reference)

### extension/manifest.json (~40 lines)
Chrome Manifest V3 metadata. Permissions: `storage`, `activeTab`, `scripting`. Host permission for bridge `localhost:9090`. Content script on all URLs at `document_idle`.

### extension/background.js (~160 lines)
Service worker. Manages config in `chrome.storage.local`, calls bridge API via `fetch()`, routes messages from popup/content script, updates badge (lime "ON" / red "OFF"), health checks via `chrome.alarms`.

### extension/content.js (~200 lines)
Content script. Scans DOM for `veriqid-*` element IDs. Injects floating badge using Veriqid design tokens (dark teal, lime accent, rounded). Generates proofs via background worker. Auto-fills fields with lime highlight animation. `MutationObserver` for SPAs.

### extension/popup.html + popup.css + popup.js (~500 lines total)
Dark teal popup UI with three screens: Setup, Main (status cards), Create Identity. Uses `--vq-accent: #B8F12A` throughout. Pill-shaped lime buttons. All state via `chrome.storage.local`.

### static/base.css (~400 lines)
Full Veriqid design system for server-rendered pages. Dark teal backgrounds, lime accents, pill buttons (NOT glassy), stat badges, feature cards with highlighted variant, split-layout forms, dashboard grid, result page. Responsive down to 768px.

### templates/*.html (5 files)
Redesigned Go HTML templates using `base.css`. Nav with brand logo + links. Hero with stat badges and review component. Split-layout signup/login forms with extension auto-fill hooks. Dashboard with stat cards. Result page with success/failure states.

---

## How the Extension Connects to Other Phases

### Phase 2 (Bridge API)
Background worker calls: `GET /api/status`, `POST /api/identity/create`, `POST /api/identity/register`, `POST /api/identity/auth`.

### Phase 4 (Veriqid Server)
Content script detects `veriqid-*` fields in server templates. The redesigned templates in this phase replace the Phase 4 originals.

### Phase 6 (Parent Dashboard)
Popup includes placeholder for parent controls. Phase 6 builds the full web dashboard — will reuse `base.css` design system.

### Phase 7 (Demo Platform)
The demo "KidsTube" IS the Veriqid server with these redesigned templates. Phase 7 adds video content placeholders and polish on top.

### Phase 8 (JS SDK)
Third-party platforms integrating the SDK will include `veriqid-*` fields. The extension works automatically on any page with these fields.

---

## Next Steps: Phase 6 — Parent Dashboard

With Phase 5 complete, the end-to-end user flow is fully automated and branded. Phase 6 creates a web dashboard (reusing `base.css`) where parents can view their child's registered platforms, see contract events, and revoke the master identity via MetaMask.
