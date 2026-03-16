// ============================================================
// Veriqid Popup — UI Controller
// ============================================================
// Manages screen transitions, bridge health display, identity
// creation, key switching, and settings persistence.
// Talks to the background service worker via chrome.runtime.
// ============================================================

(function () {
  "use strict";

  // ---- DOM References ----

  const $ = (id) => document.getElementById(id);

  const screens = {
    setup:     $("screen-setup"),
    main:      $("screen-main"),
    create:    $("screen-create"),
    changeKey: $("screen-change-key"),
  };

  // Header
  const statusDot = $("status-dot");

  // Setup screen
  const setupBridgeUrl = $("setup-bridge-url");
  const setupKeypath   = $("setup-keypath");
  const setupEthkey    = $("setup-ethkey");
  const btnSetupSave   = $("btn-setup-save");
  const setupError     = $("setup-error");

  // Main screen
  const bridgeStatus   = $("bridge-status");
  const bridgeContract = $("bridge-contract");
  const bridgeIdCount  = $("bridge-id-count");
  const bridgeVersion  = $("bridge-version");
  const activeKeypath  = $("active-keypath");
  const btnRefresh     = $("btn-refresh");
  const btnCreateId    = $("btn-create-identity");
  const btnChangeKey   = $("btn-change-key");
  const btnSettings    = $("btn-settings");
  const footerVersion  = $("footer-version");
  const toggleAutoFill   = $("toggle-autofill");
  const toggleAutoSubmit = $("toggle-autosubmit");

  // Create Identity screen
  const createKeypath    = $("create-keypath");
  const btnCreateConfirm = $("btn-create-confirm");
  const btnCreateCancel  = $("btn-create-cancel");
  const createResult     = $("create-result");
  const createMpk        = $("create-mpk");
  const createIndex      = $("create-index");
  const btnCreateUse     = $("btn-create-use");
  const createError      = $("create-error");

  // Change Key screen
  const changeKeypath    = $("change-keypath");
  const btnChangeConfirm = $("btn-change-confirm");
  const btnChangeCancel  = $("btn-change-cancel");

  // ============================================================
  // SCREEN MANAGEMENT
  // ============================================================

  function showScreen(name) {
    Object.values(screens).forEach((s) => (s.style.display = "none"));
    if (screens[name]) screens[name].style.display = "block";
  }

  // ============================================================
  // HELPERS
  // ============================================================

  function truncate(str, len = 20) {
    if (!str) return "—";
    if (str.length <= len) return str;
    return str.slice(0, len / 2) + "..." + str.slice(-len / 2);
  }

  function showError(el, msg) {
    el.textContent = msg;
    el.style.display = "block";
  }

  function hideError(el) {
    el.textContent = "";
    el.style.display = "none";
  }

  function setLoading(btn, loading) {
    if (loading) {
      btn.classList.add("loading");
      btn.disabled = true;
    } else {
      btn.classList.remove("loading");
      btn.disabled = false;
    }
  }

  /** Send a message to the background service worker and return the response. */
  function bg(message) {
    return new Promise((resolve, reject) => {
      chrome.runtime.sendMessage(message, (response) => {
        if (chrome.runtime.lastError) {
          reject(new Error(chrome.runtime.lastError.message));
        } else {
          resolve(response);
        }
      });
    });
  }

  // ============================================================
  // BRIDGE STATUS
  // ============================================================

  async function refreshBridgeStatus() {
    bridgeStatus.textContent = "Checking...";
    bridgeContract.textContent = "—";
    bridgeIdCount.textContent = "—";
    bridgeVersion.textContent = "—";

    try {
      const result = await bg({ action: "checkBridge" });

      if (result.connected) {
        statusDot.className = "status-dot connected";
        bridgeStatus.textContent = "Connected";
        bridgeStatus.style.color = "var(--vq-success)";

        bridgeContract.textContent = truncate(result.contract_address || result.contractAddress || "—", 24);
        bridgeIdCount.textContent = result.identity_count ?? result.identityCount ?? "—";
        bridgeVersion.textContent = result.version || "—";
      } else {
        statusDot.className = "status-dot disconnected";
        bridgeStatus.textContent = "Disconnected";
        bridgeStatus.style.color = "var(--vq-error)";
      }
    } catch (err) {
      statusDot.className = "status-dot disconnected";
      bridgeStatus.textContent = "Error";
      bridgeStatus.style.color = "var(--vq-error)";
      console.error("[Veriqid Popup] Bridge check error:", err);
    }
  }

  // ============================================================
  // SETUP FLOW
  // ============================================================

  async function handleSetupSave() {
    hideError(setupError);

    const bridgeUrl = setupBridgeUrl.value.trim();
    const keypath   = setupKeypath.value.trim();
    const ethkey    = setupEthkey.value.trim();

    // Validation
    if (!bridgeUrl) {
      showError(setupError, "Bridge URL is required.");
      return;
    }
    if (!keypath) {
      showError(setupError, "Key path is required.");
      return;
    }

    setLoading(btnSetupSave, true);

    try {
      // Save configuration
      await bg({
        action: "saveConfig",
        config: {
          bridgeUrl,
          keypath,
          ethkey,
          isSetup: true,
          autoFill: true,
          autoSubmit: false,
        },
      });

      // Verify the bridge is reachable
      const status = await bg({ action: "checkBridge" });

      if (!status.connected) {
        showError(
          setupError,
          "Settings saved, but the bridge is not reachable. Make sure it's running."
        );
        // Still proceed to main — the user can start the bridge later
      }

      // Transition to main screen
      await loadMainScreen();
      showScreen("main");
    } catch (err) {
      showError(setupError, err.message);
    } finally {
      setLoading(btnSetupSave, false);
    }
  }

  // ============================================================
  // MAIN SCREEN
  // ============================================================

  async function loadMainScreen() {
    const config = await bg({ action: "getConfig" });

    // Display active key path
    activeKeypath.textContent = config.keypath || "None";

    // Settings toggles
    toggleAutoFill.checked   = config.autoFill !== false;
    toggleAutoSubmit.checked  = config.autoSubmit === true;

    // Version footer
    const manifest = chrome.runtime.getManifest();
    footerVersion.textContent = `v${manifest.version}`;

    // Refresh bridge status
    await refreshBridgeStatus();
  }

  // ============================================================
  // CREATE IDENTITY
  // ============================================================

  let lastCreatedKeypath = "";

  async function handleCreateIdentity() {
    hideError(createError);
    createResult.style.display = "none";

    const keypath = createKeypath.value.trim();
    if (!keypath) {
      showError(createError, "Key path is required.");
      return;
    }

    setLoading(btnCreateConfirm, true);

    try {
      const config = await bg({ action: "getConfig" });

      const result = await bg({
        action: "createIdentity",
        keypath: keypath,
        ethkey: config.ethkey || "",
      });

      if (result.error) {
        throw new Error(result.error);
      }

      // Show success result
      createMpk.textContent   = truncate(result.mpk_hex || result.mpk || "—", 30);
      createIndex.textContent  = result.onchain_index ?? result.index ?? "—";
      createResult.style.display = "block";
      lastCreatedKeypath = keypath;

    } catch (err) {
      showError(createError, err.message);
    } finally {
      setLoading(btnCreateConfirm, false);
    }
  }

  async function handleUseCreatedIdentity() {
    if (!lastCreatedKeypath) return;

    await bg({
      action: "saveConfig",
      config: { keypath: lastCreatedKeypath },
    });

    await loadMainScreen();
    showScreen("main");
  }

  // ============================================================
  // CHANGE KEY
  // ============================================================

  async function handleChangeKey() {
    const keypath = changeKeypath.value.trim();
    if (!keypath) return;

    setLoading(btnChangeConfirm, true);

    try {
      await bg({
        action: "saveConfig",
        config: { keypath },
      });

      await loadMainScreen();
      showScreen("main");
    } catch (err) {
      console.error("[Veriqid Popup] Change key error:", err);
    } finally {
      setLoading(btnChangeConfirm, false);
    }
  }

  // ============================================================
  // SETTINGS TOGGLES
  // ============================================================

  async function handleToggleAutoFill() {
    await bg({
      action: "saveConfig",
      config: { autoFill: toggleAutoFill.checked },
    });
  }

  async function handleToggleAutoSubmit() {
    await bg({
      action: "saveConfig",
      config: { autoSubmit: toggleAutoSubmit.checked },
    });
  }

  // ============================================================
  // EVENT LISTENERS
  // ============================================================

  // Setup screen
  btnSetupSave.addEventListener("click", handleSetupSave);

  // Main screen
  btnRefresh.addEventListener("click", refreshBridgeStatus);

  btnCreateId.addEventListener("click", () => {
    createKeypath.value = "";
    createResult.style.display = "none";
    hideError(createError);
    showScreen("create");
  });

  btnChangeKey.addEventListener("click", () => {
    changeKeypath.value = "";
    showScreen("changeKey");
  });

  btnSettings.addEventListener("click", async () => {
    // Pre-fill setup form with current config
    const config = await bg({ action: "getConfig" });
    setupBridgeUrl.value = config.bridgeUrl || "http://127.0.0.1:9090";
    setupKeypath.value   = config.keypath || "";
    setupEthkey.value    = config.ethkey || "";
    hideError(setupError);
    showScreen("setup");
  });

  toggleAutoFill.addEventListener("change", handleToggleAutoFill);
  toggleAutoSubmit.addEventListener("change", handleToggleAutoSubmit);

  // Create Identity screen
  btnCreateConfirm.addEventListener("click", handleCreateIdentity);
  btnCreateCancel.addEventListener("click", () => showScreen("main"));
  btnCreateUse.addEventListener("click", handleUseCreatedIdentity);

  // Change Key screen
  btnChangeConfirm.addEventListener("click", handleChangeKey);
  btnChangeCancel.addEventListener("click", () => showScreen("main"));

  // ============================================================
  // INITIALIZATION
  // ============================================================

  async function init() {
    try {
      const config = await bg({ action: "getConfig" });

      if (config.isSetup) {
        await loadMainScreen();
        showScreen("main");
      } else {
        showScreen("setup");
      }
    } catch (err) {
      console.error("[Veriqid Popup] Init error:", err);
      showScreen("setup");
    }
  }

  init();
})();
