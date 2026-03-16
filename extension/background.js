// ============================================================
// Veriqid Background Service Worker (Manifest V3)
// ============================================================
// Handles:
// 1. Bridge API communication (localhost:9090)
// 2. Configuration storage (chrome.storage.local)
// 3. Message routing between popup and content scripts
// 4. Badge updates (connected/disconnected status)
// ============================================================

const DEFAULT_BRIDGE_URL = "http://127.0.0.1:9090";

// ---- Storage Helpers ----

async function getConfig() {
  const result = await chrome.storage.local.get([
    "bridgeUrl",
    "keypath",
    "isSetup",
    "ethkey",
    "autoFill",
    "autoSubmit",
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
  // All handlers are async — return true to keep channel open
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
      await saveConfig(message.config);
      return { success: true };

    // ---- Bridge Status ----
    case "checkBridge":
      return await checkBridgeStatus();

    // ---- Identity Management ----
    case "createIdentity":
      return await createIdentity(message.keypath, message.ethkey);

    // ---- Proof Generation (called by content script) ----
    case "generateRegistrationProof": {
      const config = await getConfig();
      if (!config.keypath) {
        throw new Error(
          "No identity configured. Open the Veriqid popup and complete setup."
        );
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
        throw new Error(
          "No identity configured. Open the Veriqid popup and complete setup."
        );
      }

      // Generate auth proof
      const authResult = await generateAuthProof(
        config.keypath,
        message.serviceName,
        message.challenge
      );

      // Auth endpoint doesn't return the SPK separately.
      // The login form needs the SPK to identify the user.
      // Workaround: also call register to derive the SPK.
      // In production, the bridge should have a /api/identity/derive-spk endpoint.
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
  // Set up periodic health check — must be inside a listener, not top-level
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
