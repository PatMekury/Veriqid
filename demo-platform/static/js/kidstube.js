/**
 * KidsTube client-side logic
 * Monitors Veriqid hidden fields for extension auto-fill
 */
(function() {
    'use strict';

    var proofField         = document.getElementById('veriqid-proof');
    var spkField           = document.getElementById('veriqid-spk');
    var signatureField     = document.getElementById('veriqid-signature');
    var contractIndexField = document.getElementById('veriqid-contract-index');
    var submitBtn          = document.getElementById('submit-btn');
    var statusDiv          = document.getElementById('veriqid-status');

    if (!proofField || !submitBtn) return; // Not on auth page

    // Check if this is a login form (has signature field) or signup (has proof field)
    var isLogin = window.location.pathname.indexOf('login') !== -1;

    /**
     * Poll hidden fields for extension auto-fill.
     * The Phase 5 content script writes to these fields directly.
     */
    function checkExtensionFill() {
        var filled = isLogin
            ? (signatureField && signatureField.value.length > 0 && spkField.value.length > 0)
            : (proofField.value.length > 0 && spkField.value.length > 0);

        if (filled) {
            // Extension has filled in the proof!
            submitBtn.disabled = false;

            if (statusDiv) {
                statusDiv.classList.add('ready');
                var iconEl = statusDiv.querySelector('.status-icon');
                var strongEl = statusDiv.querySelector('strong');
                var pEl = statusDiv.querySelector('p');

                if (iconEl) iconEl.textContent = '\u2705';
                if (strongEl) strongEl.textContent = 'Veriqid identity verified!';
                if (pEl) pEl.textContent = 'Your proof has been generated. Click the button below to continue.';
            }

            console.log('[KidsTube] Veriqid proof detected — submit enabled');
            return; // Stop polling
        }

        // Keep polling
        setTimeout(checkExtensionFill, 500);
    }

    // Also listen for input events (in case the extension triggers them)
    var fields = [proofField, spkField, signatureField, contractIndexField];
    for (var i = 0; i < fields.length; i++) {
        var field = fields[i];
        if (field) {
            field.addEventListener('input', checkExtensionFill);
            // MutationObserver for value changes via JS (extension uses .value = ...)
            var observer = new MutationObserver(checkExtensionFill);
            observer.observe(field, { attributes: true, attributeFilter: ['value'] });
        }
    }

    // Start polling
    setTimeout(checkExtensionFill, 1000);

    // Allow manual submission for testing (if no extension installed)
    // After 10 seconds, show a manual mode hint
    setTimeout(function() {
        if (submitBtn.disabled && statusDiv) {
            var hint = document.createElement('p');
            hint.style.cssText = 'font-size:12px; color:#999; margin-top:8px;';
            hint.textContent = 'Extension not detected. For testing: use the bridge CLI to generate proof values and paste them into the browser console.';
            statusDiv.appendChild(hint);
        }
    }, 10000);

})();
