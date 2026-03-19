// ============================================================
// Veriqid Portable — Popup Script
// ============================================================
// Handles mnemonic phrase input → decode → SHA-256 → MSK hex
// Stores msk_hex in chrome.storage.local for use by background.js
// ============================================================

// The same 256-word list used by the Go server (internal/mnemonic/mnemonic.go).
// Each word maps to one byte value (0–255). Order must match exactly.
const WORDLIST = [
  // 0x00–0x0F
  "apple", "arrow", "badge", "beach", "bird", "bloom", "boat", "brave",
  "brick", "bridge", "bright", "brook", "brush", "cabin", "candy", "cape",
  // 0x10–0x1F
  "castle", "chain", "chalk", "chase", "chest", "cliff", "cloud", "coast",
  "comet", "coral", "crane", "creek", "crown", "crystal", "dance", "dawn",
  // 0x20–0x2F
  "delta", "desert", "dock", "dolphin", "dragon", "dream", "drift", "drum",
  "eagle", "earth", "ember", "falcon", "field", "flame", "flash", "float",
  // 0x30–0x3F
  "flood", "flute", "forest", "forge", "fossil", "frost", "garden", "gate",
  "gem", "glade", "globe", "golden", "grain", "grape", "grass", "grove",
  // 0x40–0x4F
  "harbor", "hawk", "haven", "heart", "hedge", "hero", "hill", "hollow",
  "honey", "horizon", "horse", "house", "island", "ivory", "jade", "jewel",
  // 0x50–0x5F
  "jungle", "kite", "knight", "lake", "lamp", "lark", "leaf", "legend",
  "lemon", "light", "lily", "lion", "lodge", "lotus", "lunar", "magic",
  // 0x60–0x6F
  "maple", "marble", "marsh", "meadow", "melody", "mesa", "meteor", "mint",
  "mirror", "moon", "moss", "mountain", "mural", "nebula", "nest", "noble",
  // 0x70–0x7F
  "north", "nova", "oak", "oasis", "ocean", "olive", "orbit", "orchid",
  "osprey", "otter", "owl", "palm", "panda", "panther", "path", "pearl",
  // 0x80–0x8F
  "pebble", "pepper", "phoenix", "pilot", "pine", "pixel", "plain", "planet",
  "plum", "polar", "pond", "prism", "pulse", "puzzle", "quail", "quartz",
  // 0x90–0x9F
  "quest", "rabbit", "rain", "rainbow", "raven", "reef", "ridge", "ring",
  "river", "robin", "rocket", "rose", "ruby", "sage", "sail", "salmon",
  // 0xA0–0xAF
  "sand", "sapphire", "scout", "seed", "shadow", "shark", "shell", "shield",
  "shore", "silver", "sky", "slate", "snow", "solar", "spark", "spear",
  // 0xB0–0xBF
  "spiral", "spring", "spruce", "square", "star", "steam", "steel", "stone",
  "storm", "stream", "summit", "sun", "surf", "swan", "swift", "sword",
  // 0xC0–0xCF
  "terra", "thistle", "thorn", "thunder", "tide", "tiger", "timber", "torch",
  "tower", "trail", "tree", "tropic", "tulip", "tunnel", "turtle", "valley",
  // 0xD0–0xDF
  "vapor", "vault", "velvet", "vine", "violet", "vista", "voice", "volcano",
  "voyage", "walnut", "wave", "whale", "wheat", "willow", "wind", "winter",
  // 0xE0–0xEF
  "wolf", "wonder", "wood", "wren", "yarn", "yew", "zenith", "zephyr",
  "zinc", "atlas", "aurora", "blaze", "breeze", "canyon", "cedar", "cider",
  // 0xF0–0xFF
  "crest", "dusk", "fern", "flint", "glacier", "glow", "heron", "indigo",
  "jasper", "lava", "mangrove", "nectar", "onyx", "opal", "peak", "rapid",
];

// Build reverse lookup: word → byte index
const WORD_INDEX = {};
WORDLIST.forEach((word, i) => { WORD_INDEX[word] = i; });

/**
 * Decode a 12-word mnemonic phrase to 12 entropy bytes.
 * Returns { entropy: Uint8Array(12) } or throws on error.
 */
function decodeMnemonic(phrase) {
  const words = phrase.trim().toLowerCase().split(/\s+/);
  if (words.length !== 12) {
    throw new Error(`Expected 12 words, got ${words.length}`);
  }

  const entropy = new Uint8Array(12);
  for (let i = 0; i < 12; i++) {
    const idx = WORD_INDEX[words[i]];
    if (idx === undefined) {
      throw new Error(`Unknown word at position ${i + 1}: "${words[i]}"`);
    }
    entropy[i] = idx;
  }
  return entropy;
}

/**
 * SHA-256 hash of a Uint8Array → hex string.
 * Uses the Web Crypto API (available in extension contexts).
 */
async function sha256Hex(data) {
  const hashBuffer = await crypto.subtle.digest("SHA-256", data);
  const hashArray = new Uint8Array(hashBuffer);
  return Array.from(hashArray).map(b => b.toString(16).padStart(2, "0")).join("");
}

// ============================================================
// UI Logic
// ============================================================

async function activateKey() {
  const input = document.getElementById("mnemonic-input");
  const errorEl = document.getElementById("setup-error");
  errorEl.style.display = "none";

  const phrase = input.value.trim();
  if (!phrase) {
    errorEl.textContent = "Please paste your 12-word phrase.";
    errorEl.style.display = "block";
    return;
  }

  try {
    // Decode mnemonic → 12 entropy bytes
    const entropy = decodeMnemonic(phrase);

    // SHA-256(entropy) → 32-byte MSK hex
    const mskHex = await sha256Hex(entropy);

    // Store in chrome.storage.local
    await chrome.storage.local.set({
      mskHex: mskHex,
      isSetup: true,
    });

    console.log("[Veriqid Portable] Key activated — MSK stored (first 8):", mskHex.slice(0, 8));

    // Switch to active screen
    showActiveScreen();

  } catch (err) {
    errorEl.textContent = err.message;
    errorEl.style.display = "block";
  }
}

async function clearKey() {
  await chrome.storage.local.remove(["mskHex", "isSetup"]);
  document.getElementById("screen-active").style.display = "none";
  document.getElementById("screen-setup").style.display = "block";
  document.getElementById("mnemonic-input").value = "";
}

async function showActiveScreen() {
  document.getElementById("screen-setup").style.display = "none";
  document.getElementById("screen-active").style.display = "block";

  // Check bridge status
  try {
    const result = await chrome.runtime.sendMessage({ action: "checkBridge" });
    const statusEl = document.getElementById("bridge-status");
    if (result.connected) {
      statusEl.textContent = "Connected";
      statusEl.className = "status-value connected";
    } else {
      statusEl.textContent = "Not running";
      statusEl.className = "status-value disconnected";
    }
  } catch (e) {
    const statusEl = document.getElementById("bridge-status");
    statusEl.textContent = "Error";
    statusEl.className = "status-value disconnected";
  }

  const keyStatusEl = document.getElementById("key-status");
  keyStatusEl.textContent = "Loaded";
  keyStatusEl.className = "status-value connected";
}

// ── Init: check if already set up, wire up buttons ──
document.addEventListener("DOMContentLoaded", async () => {
  // Wire up buttons (inline onclick is blocked by Manifest V3 CSP)
  document.getElementById("btn-activate").addEventListener("click", activateKey);
  document.getElementById("btn-clear").addEventListener("click", clearKey);

  const result = await chrome.storage.local.get(["mskHex", "isSetup"]);
  if (result.isSetup && result.mskHex) {
    showActiveScreen();
  }
});
