// ============================================================
// Veriqid Portable — Content Script (Form Detection & Auto-Fill)
// ============================================================
// Identical detection logic to the original extension, but adapted
// to also fill contract-index and auto-fill on detection.
// ============================================================

(function () {
  "use strict";

  // ---- Veriqid Design Tokens ----
  const VQ = {
    dark:        "#0A1F2F",
    darkMid:     "#0F2B3C",
    accent:      "#B8F12A",
    accentHover: "#A3D925",
    white:       "#FFFFFF",
    gray300:     "#94A3B8",
    gray400:     "#64748B",
    error:       "#F87171",
    success:     "#4ADE80",
    warning:     "#FBBF24",
    radiusSm:    "8px",
    radiusMd:    "12px",
    shadowLg:    "0 8px 30px rgba(0,0,0,0.25)",
    shadowXl:    "0 12px 40px rgba(0,0,0,0.3)",
    font:        "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
  };

  // ============================================================
  // FORM DETECTION
  // ============================================================

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
        challenge,
        serviceName,
        fields: {
          spk: document.getElementById("veriqid-spk"),
          proof: proofEl,
          signature: document.getElementById("veriqid-signature"),
          decoySize: document.getElementById("veriqid-decoy-size"),
          nullifier: document.getElementById("veriqid-nullifier"),
          contractIndex: document.getElementById("veriqid-contract-index"),
          ringSize: document.getElementById("veriqid-ring-size"),
        },
      };
    } else if (authProofEl) {
      return {
        type: "login",
        challenge,
        serviceName,
        fields: {
          spk: document.getElementById("veriqid-spk"),
          authProof: authProofEl,
        },
      };
    }
    return null;
  }

  // ============================================================
  // BADGE UI
  // ============================================================

  function injectVeriqidBadge(formInfo) {
    if (document.getElementById("veriqid-badge")) return;

    const typeLabel = formInfo.type === "signup" ? "sign-up" : "login";

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
        padding: 16px 22px;
        border-radius: ${VQ.radiusMd};
        font-family: ${VQ.font};
        font-size: 14px;
        box-shadow: ${VQ.shadowLg};
        z-index: 999999;
        cursor: pointer;
        display: flex;
        align-items: center;
        gap: 14px;
        transition: transform 0.2s ease, box-shadow 0.2s ease;
        max-width: 340px;
      ">
        <div style="
          width: 40px; height: 40px;
          background: rgba(184,241,42,0.12);
          border-radius: ${VQ.radiusSm};
          display: flex; align-items: center; justify-content: center;
          flex-shrink: 0;
        "><svg width="22" height="22" viewBox="0 0 24 24" fill="none"><path d="M12 2L3 7v5c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5z" fill="${VQ.accent}" opacity="0.9"/><path d="M10 15.5l-3.5-3.5 1.41-1.41L10 12.67l5.59-5.59L17 8.5l-7 7z" fill="${VQ.dark}"/></svg></div>
        <div style="flex: 1; min-width: 0;">
          <div style="font-weight: 600; font-size: 14px; margin-bottom: 3px; color: ${VQ.white};">
            Veriqid <span style="color: ${VQ.accent};">detected</span>
          </div>
          <div id="veriqid-badge-text" style="
            font-size: 12px; color: ${VQ.gray300};
            white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
          ">Generating proof for ${typeLabel}...</div>
        </div>
      </div>
    `;
    document.body.appendChild(badge);

    const inner = document.getElementById("veriqid-badge-inner");
    inner.addEventListener("mouseenter", () => {
      inner.style.transform = "translateY(-2px)";
      inner.style.boxShadow = VQ.shadowXl;
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
    const colors = { loading: VQ.warning, success: VQ.success, error: VQ.error };
    const color = colors[status];
    if (color) {
      inner.style.borderColor = color;
      text.style.color = color;
    }
  }

  function dismissBadge(delayMs) {
    setTimeout(() => {
      const badge = document.getElementById("veriqid-badge");
      if (badge) {
        badge.style.transition = "opacity 0.5s ease";
        badge.style.opacity = "0";
        setTimeout(() => badge.remove(), 500);
      }
    }, delayMs);
  }

  // ============================================================
  // AUTO-FILL LOGIC
  // ============================================================

  async function handleAutoFill(formInfo) {
    try {
      updateBadge("loading", "Generating proof...");

      if (formInfo.type === "signup") {
        await handleSignupAutoFill(formInfo);
      } else if (formInfo.type === "login") {
        await handleLoginAutoFill(formInfo);
      }

      updateBadge("success", "Auto-filled successfully!");

      // Enable the submit button if it's disabled
      const submitBtn = document.getElementById("submit-btn");
      if (submitBtn) submitBtn.disabled = false;

      // Check auto-submit
      const config = await chrome.runtime.sendMessage({ action: "getConfig" });
      if (config && config.autoSubmit) {
        setTimeout(() => {
          const form = findParentForm(formInfo);
          if (form) form.submit();
        }, 500);
      }

      dismissBadge(3000);

    } catch (error) {
      console.error("[Veriqid Portable] Auto-fill error:", error);
      updateBadge("error", error.message);
    }
  }

  async function handleSignupAutoFill(formInfo) {
    const response = await chrome.runtime.sendMessage({
      action: "generateRegistrationProof",
      challenge: formInfo.challenge,
      serviceName: formInfo.serviceName,
    });

    if (response.error && !response.proof_hex) {
      throw new Error(response.error);
    }

    if (!response.spk_hex || !response.proof_hex) {
      throw new Error("Incomplete response from bridge.");
    }

    // Fill the form fields
    setFieldValue(formInfo.fields.spk, response.spk_hex);
    setFieldValue(formInfo.fields.proof, response.proof_hex);
    if (formInfo.fields.ringSize) {
      setFieldValue(formInfo.fields.ringSize, String(response.ring_size || ""));
    }
    // Also fill decoy-size (KidsTube reads this field for verification)
    if (formInfo.fields.decoySize) {
      setFieldValue(formInfo.fields.decoySize, String(response.ring_size || ""));
    }

    // Fill contract_index if available in the response
    // The bridge register response includes the signer's index
    if (formInfo.fields.contractIndex && response.contract_index !== undefined) {
      setFieldValue(formInfo.fields.contractIndex, String(response.contract_index));
    }

    console.log("[Veriqid Portable] Signup form auto-filled:", {
      spk: response.spk_hex?.slice(0, 16) + "...",
      proof: "present",
      ringSize: response.ring_size,
    });
  }

  async function handleLoginAutoFill(formInfo) {
    const response = await chrome.runtime.sendMessage({
      action: "generateAuthProof",
      challenge: formInfo.challenge,
      serviceName: formInfo.serviceName,
    });

    if (response.error && !response.auth_proof_hex) {
      throw new Error(response.error);
    }

    setFieldValue(formInfo.fields.authProof, response.auth_proof_hex);
    if (response.spk_hex && formInfo.fields.spk) {
      setFieldValue(formInfo.fields.spk, response.spk_hex);
    }

    console.log("[Veriqid Portable] Login form auto-filled");
  }

  // ============================================================
  // HELPERS
  // ============================================================

  function setFieldValue(element, value) {
    if (!element || !value) return;
    element.value = value;
    element.dispatchEvent(new Event("input", { bubbles: true }));
    element.dispatchEvent(new Event("change", { bubbles: true }));

    const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype, "value"
    )?.set;
    if (nativeInputValueSetter) {
      nativeInputValueSetter.call(element, value);
      element.dispatchEvent(new Event("input", { bubbles: true }));
    }
  }

  function findParentForm(formInfo) {
    const fields = Object.values(formInfo.fields);
    for (const field of fields) {
      if (field && field.closest) {
        const form = field.closest("form");
        if (form) return form;
      }
    }
    return null;
  }

  // ============================================================
  // INITIALIZATION
  // ============================================================

  function init() {
    const formInfo = detectVeriqidForm();
    if (!formInfo) return;

    console.log(`[Veriqid Portable] Detected ${formInfo.type} form`);

    chrome.runtime.sendMessage({
      action: "pageHasVeriqidForm",
      formType: formInfo.type,
    });

    injectVeriqidBadge(formInfo);

    // Auto-fill immediately (portable extension always auto-fills)
    handleAutoFill(formInfo);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }

  // ── SPA Support ──
  const observer = new MutationObserver(() => {
    if (document.getElementById("veriqid-badge")) return;
    const formInfo = detectVeriqidForm();
    if (formInfo) {
      observer.disconnect();
      console.log(`[Veriqid Portable] Detected ${formInfo.type} form (dynamic)`);
      chrome.runtime.sendMessage({ action: "pageHasVeriqidForm", formType: formInfo.type });
      injectVeriqidBadge(formInfo);
      handleAutoFill(formInfo);
    }
  });

  observer.observe(document.body || document.documentElement, {
    childList: true,
    subtree: true,
  });

  setTimeout(() => observer.disconnect(), 10000);
})();
