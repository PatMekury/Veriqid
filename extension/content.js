// ============================================================
// Veriqid Content Script — Form Detection & Auto-Fill
// ============================================================
// Runs on every page. Detects Veriqid-enabled forms by scanning
// for hidden fields with specific IDs (veriqid-challenge, etc.),
// injects a floating badge using the Veriqid design system,
// and auto-fills proof fields via the bridge API.
// ============================================================

(function () {
  "use strict";

  // ---- Veriqid Design Tokens (inline — no CSS imports in content scripts) ----

  const VQ = {
    dark:        "#0A1F2F",
    darkMid:     "#0F2B3C",
    darkLight:   "#153A4F",
    accent:      "#B8F12A",
    accentHover: "#A3D925",
    white:       "#FFFFFF",
    gray200:     "#CBD5E1",
    gray300:     "#94A3B8",
    gray400:     "#64748B",
    error:       "#F87171",
    success:     "#4ADE80",
    warning:     "#FBBF24",
    radiusSm:    "8px",
    radiusMd:    "12px",
    radiusFull:  "9999px",
    shadowLg:    "0 8px 30px rgba(0,0,0,0.25)",
    shadowXl:    "0 12px 40px rgba(0,0,0,0.3)",
    font:        "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
  };

  // ============================================================
  // FORM DETECTION
  // ============================================================

  /**
   * Scans the DOM for Veriqid form fields by element ID.
   * Returns null if this is not a Veriqid-enabled page.
   * Returns an object describing the form type and field references.
   */
  function detectVeriqidForm() {
    const challengeEl = document.getElementById("veriqid-challenge");
    const serviceNameEl = document.getElementById("veriqid-service-name");

    // Not a Veriqid page — exit immediately
    if (!challengeEl || !serviceNameEl) return null;

    const challenge = challengeEl.value;
    const serviceName = serviceNameEl.value;

    // Fields exist but are empty — server didn't inject values
    if (!challenge || !serviceName) return null;

    // Determine form type by which proof field exists
    const proofEl = document.getElementById("veriqid-proof");
    const authProofEl = document.getElementById("veriqid-auth-proof");

    if (proofEl) {
      return {
        type: "signup",
        challenge: challenge,
        serviceName: serviceName,
        fields: {
          spk: document.getElementById("veriqid-spk"),
          proof: proofEl,
          ringSize: document.getElementById("veriqid-ring-size"),
        },
      };
    } else if (authProofEl) {
      return {
        type: "login",
        challenge: challenge,
        serviceName: serviceName,
        fields: {
          spk: document.getElementById("veriqid-spk"),
          authProof: authProofEl,
        },
      };
    }

    return null;
  }

  // ============================================================
  // BADGE UI — Veriqid Design System
  // ============================================================

  /**
   * Injects a floating badge in the bottom-right corner.
   * Uses the Veriqid design: dark teal bg, lime accent, rounded card.
   */
  function injectVeriqidBadge(formInfo) {
    // Don't inject twice
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
        <!-- Shield icon container -->
        <div style="
          width: 40px;
          height: 40px;
          background: rgba(184,241,42,0.12);
          border-radius: ${VQ.radiusSm};
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 20px;
          flex-shrink: 0;
        ">🛡️</div>

        <!-- Text content -->
        <div style="flex: 1; min-width: 0;">
          <div style="
            font-weight: 600;
            font-size: 14px;
            margin-bottom: 3px;
            color: ${VQ.white};
          ">
            Veriqid <span style="color: ${VQ.accent};">detected</span>
          </div>
          <div id="veriqid-badge-text" style="
            font-size: 12px;
            color: ${VQ.gray300};
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
          ">Click to auto-fill ${typeLabel} form</div>
        </div>

        <!-- Chevron -->
        <div style="
          font-size: 16px;
          color: ${VQ.gray400};
          flex-shrink: 0;
        ">→</div>
      </div>
    `;

    document.body.appendChild(badge);

    // Hover effects
    const inner = document.getElementById("veriqid-badge-inner");
    inner.addEventListener("mouseenter", () => {
      inner.style.transform = "translateY(-2px)";
      inner.style.boxShadow = VQ.shadowXl;
    });
    inner.addEventListener("mouseleave", () => {
      inner.style.transform = "translateY(0)";
      inner.style.boxShadow = VQ.shadowLg;
    });

    // Click triggers auto-fill
    inner.addEventListener("click", () => handleAutoFill(formInfo));
  }

  /**
   * Updates the badge text and border color based on status.
   */
  function updateBadge(status, message) {
    const text = document.getElementById("veriqid-badge-text");
    const inner = document.getElementById("veriqid-badge-inner");
    if (!text || !inner) return;

    text.textContent = message;

    const colors = {
      loading: VQ.warning,
      success: VQ.success,
      error: VQ.error,
    };

    const color = colors[status];
    if (color) {
      inner.style.borderColor = color;
      text.style.color = color;
    }
  }

  /**
   * Fades out and removes the badge after a delay.
   */
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

  /**
   * Main auto-fill handler. Generates proof via background worker
   * and fills the form fields.
   */
  async function handleAutoFill(formInfo) {
    try {
      updateBadge("loading", "Generating proof...");

      if (formInfo.type === "signup") {
        await handleSignupAutoFill(formInfo);
      } else if (formInfo.type === "login") {
        await handleLoginAutoFill(formInfo);
      }

      updateBadge("success", "Auto-filled successfully!");

      // Check if auto-submit is enabled
      const config = await chrome.runtime.sendMessage({ action: "getConfig" });
      if (config && config.autoSubmit) {
        setTimeout(() => {
          const form = findParentForm(formInfo);
          if (form) {
            console.log("[Veriqid] Auto-submitting form");
            form.submit();
          }
        }, 500);
      }

      // Fade out badge after 3 seconds
      dismissBadge(3000);

    } catch (error) {
      console.error("[Veriqid] Auto-fill error:", error);
      updateBadge("error", error.message);
    }
  }

  /**
   * Signup: calls bridge /api/identity/register, fills spk + proof + ring_size.
   */
  async function handleSignupAutoFill(formInfo) {
    const response = await chrome.runtime.sendMessage({
      action: "generateRegistrationProof",
      challenge: formInfo.challenge,
      serviceName: formInfo.serviceName,
    });

    // Check for errors — bridge returns {success: false, error: "..."} on failure
    if (response.error && !response.proof_hex) {
      throw new Error(response.error);
    }

    // Validate required fields before filling
    if (!response.spk_hex || !response.proof_hex || !response.ring_size) {
      throw new Error(
        "Incomplete response from bridge. Missing: " +
        [!response.spk_hex && "spk", !response.proof_hex && "proof", !response.ring_size && "ring_size"]
          .filter(Boolean).join(", ")
      );
    }

    // Fill the form fields
    setFieldValue(formInfo.fields.spk, response.spk_hex);
    setFieldValue(formInfo.fields.proof, response.proof_hex);
    setFieldValue(formInfo.fields.ringSize, String(response.ring_size));

    console.log("[Veriqid] Signup form auto-filled:", {
      spk: response.spk_hex ? response.spk_hex.slice(0, 16) + "..." : "missing",
      proof: response.proof_hex ? "present" : "missing",
      ringSize: response.ring_size,
    });
  }

  /**
   * Login: calls bridge /api/identity/auth, fills auth_proof + spk.
   */
  async function handleLoginAutoFill(formInfo) {
    const response = await chrome.runtime.sendMessage({
      action: "generateAuthProof",
      challenge: formInfo.challenge,
      serviceName: formInfo.serviceName,
    });

    if (response.error && !response.auth_proof_hex) {
      throw new Error(response.error);
    }

    // Fill auth proof
    setFieldValue(formInfo.fields.authProof, response.auth_proof_hex);

    // Fill SPK if available (background worker derives it via register call)
    if (response.spk_hex && formInfo.fields.spk) {
      setFieldValue(formInfo.fields.spk, response.spk_hex);
    }

    console.log("[Veriqid] Login form auto-filled:", {
      authProof: response.auth_proof_hex ? "present" : "missing",
      spk: response.spk_hex ? response.spk_hex.slice(0, 16) + "..." : "missing",
    });
  }

  // ============================================================
  // HELPERS
  // ============================================================

  /**
   * Sets an input field's value and dispatches events so that
   * JavaScript frameworks (React, Vue, Angular) detect the change.
   * Also applies a brief lime-green highlight animation.
   */
  function setFieldValue(element, value) {
    if (!element || !value) return;

    // Set the value
    element.value = value;

    // Dispatch native events for framework compatibility
    // React uses synthetic events — it needs the native input event to trigger onChange
    element.dispatchEvent(new Event("input", { bubbles: true }));
    element.dispatchEvent(new Event("change", { bubbles: true }));

    // For React 16+ internal value tracking
    const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype,
      "value"
    )?.set;
    if (nativeInputValueSetter) {
      nativeInputValueSetter.call(element, value);
      element.dispatchEvent(new Event("input", { bubbles: true }));
    }

    // Visual feedback — lime green highlight flash
    const origBg = element.style.backgroundColor;
    const origBorder = element.style.borderColor;
    const origTransition = element.style.transition;

    element.style.transition = "background-color 0.3s ease, border-color 0.3s ease";
    element.style.backgroundColor = "rgba(184, 241, 42, 0.1)";
    element.style.borderColor = VQ.accent;

    setTimeout(() => {
      element.style.backgroundColor = origBg;
      element.style.borderColor = origBorder;
      // Restore original transition after animation completes
      setTimeout(() => {
        element.style.transition = origTransition;
      }, 300);
    }, 1500);
  }

  /**
   * Finds the closest <form> ancestor of any of the Veriqid fields.
   */
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

  /**
   * Main entry point. Checks for Veriqid forms, notifies background,
   * injects badge, and optionally auto-fills immediately.
   */
  function init() {
    const formInfo = detectVeriqidForm();

    if (!formInfo) {
      // Not a Veriqid-enabled page — exit silently, no CPU wasted
      return;
    }

    console.log(`[Veriqid] Detected ${formInfo.type} form on this page`);

    // Notify the background service worker so it can update the tab badge
    chrome.runtime.sendMessage({
      action: "pageHasVeriqidForm",
      formType: formInfo.type,
    });

    // Inject the floating auto-fill badge
    injectVeriqidBadge(formInfo);

    // If auto-fill is enabled, fill immediately without waiting for click
    chrome.runtime.sendMessage({ action: "getConfig" }, (config) => {
      if (chrome.runtime.lastError) {
        console.warn("[Veriqid] Could not get config:", chrome.runtime.lastError.message);
        return;
      }
      if (config && config.autoFill) {
        console.log("[Veriqid] Auto-fill enabled — filling immediately");
        handleAutoFill(formInfo);
      }
    });
  }

  // Run detection when DOM is ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    // DOM already parsed (run_at: document_idle)
    init();
  }

  // ============================================================
  // SPA SUPPORT — MutationObserver
  // ============================================================
  // Single-page applications may load form content dynamically
  // after the initial page load. Watch for DOM changes and re-run
  // detection when new elements appear.

  const observer = new MutationObserver((mutations) => {
    // If we already injected a badge, stop — form was already found
    if (document.getElementById("veriqid-badge")) return;

    const formInfo = detectVeriqidForm();
    if (formInfo) {
      observer.disconnect();
      console.log(`[Veriqid] Detected ${formInfo.type} form (dynamically loaded)`);

      chrome.runtime.sendMessage({
        action: "pageHasVeriqidForm",
        formType: formInfo.type,
      });

      injectVeriqidBadge(formInfo);

      // Check auto-fill for dynamically loaded forms too
      chrome.runtime.sendMessage({ action: "getConfig" }, (config) => {
        if (chrome.runtime.lastError) return;
        if (config && config.autoFill) {
          handleAutoFill(formInfo);
        }
      });
    }
  });

  // Observe the entire document for added child elements
  observer.observe(document.body || document.documentElement, {
    childList: true,
    subtree: true,
  });

  // Stop observing after 10 seconds to avoid unnecessary CPU usage
  // on pages that never load Veriqid forms
  setTimeout(() => observer.disconnect(), 10000);

})();
