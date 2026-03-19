// ============================================================
// Veriqid Portable — Background Service Worker (Manifest V3)
// ============================================================
// Sends msk_hex (from mnemonic) to the bridge instead of keypath.
// Bridge endpoint accepts either msk_hex or keypath — we always use msk_hex.
// ============================================================

const DEFAULT_BRIDGE_URL = "http://127.0.0.1:9090";

// ---- Storage Helpers ----

async function getConfig() {
  const result = await chrome.storage.local.get([
    "bridgeUrl",
    "mskHex",
    "isSetup",
    "autoFill",
    "autoSubmit",
  ]);
  return {
    bridgeUrl: result.bridgeUrl || DEFAULT_BRIDGE_URL,
    mskHex: result.mskHex || "",
    isSetup: result.isSetup || false,
    autoFill: result.autoFill !== undefined ? result.autoFill : true,
    autoSubmit: result.autoSubmit !== undefined ? result.autoSubmit : false,
  };
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
    if (
      error.message.includes("Failed to fetch") ||
      error.message.includes("NetworkError")
    ) {
      throw new Error(
        "Bridge is not running. Start the bridge server on port 9090."
      );
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

// ---- Proof Generation (uses msk_hex instead of keypath) ----

async function generateRegistrationProof(mskHex, serviceName, challenge) {
  return await bridgeFetch("/api/identity/register", {
    method: "POST",
    body: JSON.stringify({
      msk_hex: mskHex,
      service_name: serviceName,
      challenge: challenge,
    }),
  });
}

async function generateAuthProof(mskHex, serviceName, challenge) {
  return await bridgeFetch("/api/identity/auth", {
    method: "POST",
    body: JSON.stringify({
      msk_hex: mskHex,
      service_name: serviceName,
      challenge: challenge,
    }),
  });
}

// ---- Message Handler ----

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  handleMessage(message, sender)
    .then(sendResponse)
    .catch((error) => sendResponse({ success: false, error: error.message }));
  return true;
});

async function handleMessage(message, sender) {
  switch (message.action) {
    // ---- Config ----
    case "getConfig":
      return await getConfig();

    case "saveConfig":
      await chrome.storage.local.set(message.config);
      return { success: true };

    // ---- Bridge Status ----
    case "checkBridge":
      return await checkBridgeStatus();

    // ---- Proof Generation (called by content script) ----
    case "generateRegistrationProof": {
      const config = await getConfig();
      if (!config.mskHex) {
        throw new Error(
          "No identity key loaded. Open the Veriqid popup and paste your 12-word phrase."
        );
      }
      return await generateRegistrationProof(
        config.mskHex,
        message.serviceName,
        message.challenge
      );
    }

    case "generateAuthProof": {
      const config = await getConfig();
      if (!config.mskHex) {
        throw new Error(
          "No identity key loaded. Open the Veriqid popup and paste your 12-word phrase."
        );
      }

      // Generate auth proof
      const authResult = await generateAuthProof(
        config.mskHex,
        message.serviceName,
        message.challenge
      );

      // Also derive SPK via registration endpoint (needed for login forms)
      let spkHex = "";
      try {
        const regResult = await generateRegistrationProof(
          config.mskHex,
          message.serviceName,
          message.challenge
        );
        spkHex = regResult.spk_hex || "";
      } catch (e) {
        console.warn("[Veriqid Portable] Could not derive SPK for auth:", e.message);
      }

      return { ...authResult, spk_hex: spkHex };
    }

    // ---- Page Detection (from content script) ----
    case "pageHasVeriqidForm":
      await chrome.action.setBadgeText({
        text: "V",
        tabId: sender.tab?.id,
      });
      await chrome.action.setBadgeBackgroundColor({
        color: "#B8F12A",
        tabId: sender.tab?.id,
      });
      return { success: true };

    default:
      throw new Error(`Unknown action: ${message.action}`);
  }
}

// ---- Startup & Periodic Health Checks ----

chrome.runtime.onInstalled.addListener(async () => {
  chrome.alarms.create("bridgeHealthCheck", { periodInMinutes: 1 });
  await checkBridgeStatus();
});

chrome.runtime.onStartup.addListener(async () => {
  chrome.alarms.create("bridgeHealthCheck", { periodInMinutes: 1 });
  await checkBridgeStatus();
});

chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name === "bridgeHealthCheck") {
    await checkBridgeStatus();
  }
});
