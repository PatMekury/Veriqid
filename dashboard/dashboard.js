// ═══════════════════════════════════════════════════════════════
// Veriqid Parent Portal — Client-side JavaScript
// Communicates with /api/parent/* endpoints via fetch()
// No MetaMask, no ethers.js — all blockchain interaction is server-side
// ═══════════════════════════════════════════════════════════════

// ── State ────────────────────────────────────────────────────
let currentTab = 'login';     // 'login' or 'register'
let currentMethod = 'email';  // 'email' or 'phone'
let otpSent = false;
let pendingChildID = null;    // For onboarding flow
let revokeChildID = null;     // For revoke modal

const AGE_LABELS = { 0: 'Unknown', 1: 'Under 13', 2: 'Teen (13–17)', 3: 'Adult (18+)' };
const AGE_COLORS = { 0: '#9ca3af', 1: '#fbbf24', 2: '#60a5fa', 3: '#4ade80' };
const STATUS_CONFIG = {
    pending:  { label: 'Pending', class: 'pending',  icon: '<img class="status-icon" src="/icons/Hourglass.png" alt="">' },
    verified: { label: 'Active',  class: 'active',   icon: '<img class="status-icon" src="/icons/Check.png" alt="">' },
    revoked:  { label: 'Revoked', class: 'revoked',  icon: '<img class="status-icon" src="/icons/X mark.png" alt="">' }
};

// ── Auth Tab Switching ───────────────────────────────────────
function switchAuthTab(tab) {
    currentTab = tab;
    document.querySelectorAll('.auth-tab').forEach(t => t.classList.remove('active'));
    document.querySelector(`.auth-tab[data-tab="${tab}"]`).classList.add('active');
    // Update button text
    document.getElementById('btn-email-text').textContent = tab === 'login' ? 'Log In' : 'Create Account';
    document.getElementById('btn-phone-text').textContent = otpSent ? 'Verify Code' : 'Send Code';
}

function switchMethodTab(method) {
    currentMethod = method;
    document.querySelectorAll('.method-tab').forEach(t => t.classList.remove('active'));
    document.querySelector(`.method-tab[data-method="${method}"]`).classList.add('active');
    document.getElementById('form-email').style.display = method === 'email' ? 'block' : 'none';
    document.getElementById('form-phone').style.display = method === 'phone' ? 'block' : 'none';
}

// ── Email Auth ───────────────────────────────────────────────
async function handleEmailSubmit(e) {
    e.preventDefault();
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const errorEl = document.getElementById('auth-error');
    errorEl.style.display = 'none';

    const endpoint = currentTab === 'login' ? '/api/parent/login' : '/api/parent/register';

    try {
        const res = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });

        const data = await res.json();
        if (!res.ok) {
            errorEl.textContent = data.error || 'Something went wrong.';
            errorEl.style.display = 'block';
            return;
        }

        // Success — check if parent has children
        await onLoginSuccess(email);

    } catch (err) {
        errorEl.textContent = 'Network error. Is the server running?';
        errorEl.style.display = 'block';
    }
}

// ── Phone Auth ───────────────────────────────────────────────
async function handlePhoneSubmit(e) {
    e.preventDefault();
    const phone = document.getElementById('phone').value;
    const errorEl = document.getElementById('auth-error');
    errorEl.style.display = 'none';

    if (!otpSent) {
        // Step 1: Send OTP (for both register and login)
        const endpoint = currentTab === 'login' ? '/api/parent/send-otp' : '/api/parent/register';
        const body = { phone };

        try {
            const res = await fetch(endpoint, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });

            const data = await res.json();
            if (!res.ok) {
                errorEl.textContent = data.error || 'Failed to send code.';
                errorEl.style.display = 'block';
                return;
            }

            // Show OTP input and clear any stale value
            otpSent = true;
            document.getElementById('otp-code').value = '';
            document.getElementById('otp-group').style.display = 'block';
            document.getElementById('otp-code').focus();
            document.getElementById('btn-phone-text').textContent = 'Verify Code';

        } catch (err) {
            errorEl.textContent = 'Network error.';
            errorEl.style.display = 'block';
        }

    } else {
        // Step 2: Verify OTP
        const code = document.getElementById('otp-code').value.trim();
        console.log('[DEBUG] Sending OTP verify — phone:', phone, 'code:', code);

        if (!code || code.length !== 6) {
            errorEl.textContent = 'Please enter the 6-digit code from the server terminal.';
            errorEl.style.display = 'block';
            return;
        }

        try {
            const res = await fetch('/api/parent/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ phone, code })
            });

            const data = await res.json();
            if (!res.ok) {
                errorEl.textContent = data.error || 'Invalid code.';
                errorEl.style.display = 'block';
                return;
            }

            await onLoginSuccess(phone);

        } catch (err) {
            errorEl.textContent = 'Network error.';
            errorEl.style.display = 'block';
        }
    }
}

// ── Post-Login Logic ─────────────────────────────────────────
async function onLoginSuccess(identifier) {
    // Update nav
    document.getElementById('user-info').style.display = 'flex';
    document.getElementById('user-email').textContent = identifier;

    // Check if parent has children
    const res = await fetch('/api/parent/children');
    const data = await res.json();

    if (data.children && data.children.length > 0) {
        showDashboard(data.children);
    } else {
        showOnboarding();
    }
}

// ── Screen Navigation ────────────────────────────────────────
function showOnboarding() {
    document.getElementById('auth-screen').style.display = 'none';
    document.getElementById('onboarding-screen').style.display = 'flex';
    document.getElementById('dashboard-screen').style.display = 'none';
    // Reset to step 1
    document.getElementById('onboard-step1').style.display = 'block';
    document.getElementById('onboard-step2').style.display = 'none';
    document.getElementById('onboard-step3').style.display = 'none';
}

function showDashboard(children) {
    document.getElementById('auth-screen').style.display = 'none';
    document.getElementById('onboarding-screen').style.display = 'none';
    document.getElementById('dashboard-screen').style.display = 'block';
    renderChildren(children);
    loadEvents();
}

function goToDashboard() {
    refreshDashboard();
}

// ── Add Child ────────────────────────────────────────────────
async function handleAddChild(e) {
    e.preventDefault();
    const name = document.getElementById('child-name').value;
    const ageBracket = parseInt(document.getElementById('child-age').value);

    try {
        const res = await fetch('/api/parent/child/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ display_name: name, age_bracket: ageBracket })
        });

        const data = await res.json();
        if (!res.ok) {
            alert(data.error || 'Failed to add child.');
            return;
        }

        pendingChildID = data.child_id;

        // If the server returned a mnemonic phrase, show the portable key step
        if (data.mnemonic) {
            showMnemonicStep(name, data.mnemonic);
        } else {
            // Legacy fallback: go straight to verification step
            document.getElementById('verify-child-name').textContent = name;
            document.getElementById('onboard-step1').style.display = 'none';
            document.getElementById('onboard-step2').style.display = 'block';
        }

    } catch (err) {
        alert('Network error.');
    }
}

// ── Mnemonic Display Step ────────────────────────────────────
function showMnemonicStep(childName, phrase) {
    // Hide other steps, show mnemonic step
    document.getElementById('onboard-step1').style.display = 'none';
    document.getElementById('onboard-step2').style.display = 'none';
    document.getElementById('onboard-step3').style.display = 'none';

    // Create the mnemonic display step if it doesn't exist
    let mnemonicStep = document.getElementById('onboard-step-mnemonic');
    if (!mnemonicStep) {
        mnemonicStep = document.createElement('div');
        mnemonicStep.id = 'onboard-step-mnemonic';
        mnemonicStep.className = 'onboard-step';
        document.querySelector('.onboarding-card').appendChild(mnemonicStep);
    }

    const words = phrase.split(' ');
    const wordGrid = words.map((w, i) =>
        `<div class="mnemonic-word"><span class="mnemonic-num">${i + 1}</span>${w}</div>`
    ).join('');

    mnemonicStep.innerHTML = `
        <div class="mnemonic-header">
            <div class="mnemonic-icon"><img src="/icons/Key.png" alt="Key" style="width:48px;height:48px;object-fit:contain;"></div>
            <h2>Portable Key for ${childName}</h2>
            <p>This 12-word phrase is your child's identity key. Give it to <strong>${childName}</strong> to paste into their Veriqid browser extension.</p>
        </div>

        <div class="mnemonic-box">
            <div class="mnemonic-grid">${wordGrid}</div>
        </div>

        <div class="mnemonic-actions">
            <button class="vq-btn-secondary" onclick="copyMnemonic('${phrase}')">
                Copy to Clipboard
            </button>
            <span id="copy-feedback" class="copy-feedback" style="display:none">Copied!</span>
        </div>

        <div class="mnemonic-warning">
            <strong><img src="/icons/Warning.png" alt="" style="width:14px;height:14px;vertical-align:-2px;margin-right:3px;">Important:</strong> This phrase is shown only once. If lost, you'll need to create a new key.
            The phrase gives full access to this identity — keep it private between you and your child.
        </div>

        <button class="vq-btn-primary btn-full" onclick="mnemonicConfirmed()">
            I've Saved the Phrase — Continue to Verification
        </button>
    `;
    mnemonicStep.style.display = 'block';
}

function copyMnemonic(phrase) {
    navigator.clipboard.writeText(phrase).then(() => {
        const fb = document.getElementById('copy-feedback');
        fb.style.display = 'inline';
        setTimeout(() => { fb.style.display = 'none'; }, 2000);
    });
}

function mnemonicConfirmed() {
    // Hide mnemonic step, show verification step
    document.getElementById('onboard-step-mnemonic').style.display = 'none';
    const childName = document.getElementById('onboard-step-mnemonic')
        .querySelector('h2').textContent.replace('Portable Key for ', '');
    document.getElementById('verify-child-name').textContent = childName;
    document.getElementById('onboard-step2').style.display = 'block';
}

// ── Verification ─────────────────────────────────────────────
function selectVerifyMethod(method) {
    // Highlight selected option
    document.querySelectorAll('.verify-option').forEach(o => o.classList.remove('selected'));
    event.currentTarget.classList.add('selected');

    // Show demo simulation button
    document.getElementById('verify-demo-section').style.display = 'block';
}

async function simulateVerification() {
    const btn = document.getElementById('btn-verify-approve');
    btn.disabled = true;
    btn.textContent = 'Verifying...';

    try {
        const res = await fetch('/api/parent/verify/approve', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ child_id: pendingChildID })
        });

        const data = await res.json();
        if (!res.ok) {
            alert(data.error || 'Verification failed.');
            btn.disabled = false;
            btn.textContent = 'Simulate Verification Approval';
            return;
        }

        // Show success
        const childName = document.getElementById('verify-child-name').textContent;
        document.getElementById('success-child-name').textContent = childName;
        document.getElementById('onboard-step2').style.display = 'none';
        document.getElementById('onboard-step3').style.display = 'block';

    } catch (err) {
        alert('Network error.');
        btn.disabled = false;
        btn.textContent = 'Simulate Verification Approval';
    }
}

// ── Dashboard Rendering ──────────────────────────────────────
async function refreshDashboard() {
    try {
        const res = await fetch('/api/parent/children');
        const data = await res.json();

        if (data.children) {
            showDashboard(data.children);
        }
    } catch (err) {
        console.error('Failed to refresh:', err);
    }
}

function renderChildren(children) {
    const grid = document.getElementById('children-grid');

    // Update summary
    const total = children.length;
    const active = children.filter(c => c.status === 'verified').length;
    const revoked = children.filter(c => c.status === 'revoked').length;
    document.getElementById('stat-total').textContent = total;
    document.getElementById('stat-active').textContent = active;
    document.getElementById('stat-revoked').textContent = revoked;

    if (children.length === 0) {
        grid.innerHTML = '<div class="empty-state"><h3>No children added yet.</h3></div>';
        return;
    }

    grid.innerHTML = children.map(child => {
        const status = STATUS_CONFIG[child.status] || STATUS_CONFIG.pending;
        const mpkShort = child.mpk_hex
            ? `0x${child.mpk_hex.slice(0, 8)}...${child.mpk_hex.slice(-6)}`
            : '—';

        let actionBtn = '';
        if (child.status === 'verified') {
            actionBtn = `<button class="btn-danger" onclick="openRevokeModal(${child.id}, '${child.display_name}')">Revoke Identity</button>`;
        } else if (child.status === 'pending') {
            actionBtn = `<button class="vq-btn-primary btn-sm" onclick="showOnboarding()">Complete Verification</button>`;
        } else {
            actionBtn = `<div class="btn-revoked"><img src="/icons/X mark.png" alt="" style="width:12px;height:12px;vertical-align:-1px;margin-right:4px;">Revoked</div>`;
        }

        return `
            <div class="identity-card">
                <div class="identity-card-header">
                    <div class="identity-card-title">
                        <span class="identity-index">${child.display_name}</span>
                    </div>
                    <span class="status-badge ${status.class}">${status.icon} ${status.label}</span>
                </div>
                <div class="identity-card-body">
                    <div class="identity-detail">
                        <span class="identity-detail-label">Age Range</span>
                        <span class="identity-detail-value" style="color:${AGE_COLORS[child.age_bracket]}">${child.age_bracket_label || AGE_LABELS[child.age_bracket]}</span>
                    </div>
                    <div class="identity-detail">
                        <span class="identity-detail-label">Public Key</span>
                        <span class="identity-detail-value mono">${mpkShort}</span>
                    </div>
                    <div class="identity-detail">
                        <span class="identity-detail-label">Verified</span>
                        <span class="identity-detail-value">${child.verified_at || 'Not yet'}</span>
                    </div>
                </div>
                <div class="identity-card-actions">${actionBtn}</div>
            </div>
        `;
    }).join('');
}

// ── Events ───────────────────────────────────────────────────
async function loadEvents() {
    try {
        const res = await fetch('/api/parent/events');
        const data = await res.json();
        const logEl = document.getElementById('activity-log');
        const countEl = document.getElementById('event-count');

        if (!data.events || data.events.length === 0) {
            logEl.innerHTML = '<div class="empty-state"><p>No activity yet.</p></div>';
            countEl.textContent = '0 events';
            return;
        }

        countEl.textContent = `${data.events.length} event${data.events.length !== 1 ? 's' : ''}`;
        logEl.innerHTML = data.events.map(evt => {
            const isRevoke = evt.type === 'revoked';
            const isPlatform = evt.type.startsWith('platform_');

            if (isPlatform) {
                // Platform activity events (e.g., "Alex registered on KidsTube")
                const platformEvent = evt.type.replace('platform_', '');
                const platformIcon = platformEvent === 'registered' ? '<img src="/icons/Globe.png" alt="" style="width:16px;height:16px;">' : '<img src="/icons/Warning.png" alt="" style="width:16px;height:16px;">';
                const platformLabel = platformEvent === 'registered' ? 'Joined' : platformEvent;
                const timeStr = evt.event_time ? new Date(evt.event_time).toLocaleString() : '';
                return `
                    <div class="activity-item platform-event">
                        <div class="activity-icon platform">
                            ${platformIcon}
                        </div>
                        <div class="activity-content">
                            <div class="activity-title">${evt.child_name || 'Identity'} — ${platformLabel} <strong>${evt.service_name || 'a platform'}</strong></div>
                            <div class="activity-desc">${timeStr ? timeStr + ' · ' : ''}Contract index: ${evt.contract_index}</div>
                        </div>
                    </div>
                `;
            }

            return `
                <div class="activity-item">
                    <div class="activity-icon ${isRevoke ? 'revoked' : 'registered'}">
                        ${isRevoke ? '<img src="/icons/X mark.png" alt="" style="width:14px;height:14px;">' : '<img src="/icons/Check.png" alt="" style="width:14px;height:14px;">'}
                    </div>
                    <div class="activity-content">
                        <div class="activity-title">${evt.child_name || 'Identity'} — ${isRevoke ? 'Revoked' : 'Registered'}</div>
                        <div class="activity-desc">Contract index: ${evt.contract_index} · Block #${evt.block_number}</div>
                    </div>
                </div>
            `;
        }).join('');

    } catch (err) {
        console.error('Failed to load events:', err);
    }
}

// ── Revocation ───────────────────────────────────────────────
function openRevokeModal(childID, childName) {
    revokeChildID = childID;
    document.getElementById('revoke-child-name').textContent = childName;
    document.getElementById('revoke-password').value = '';
    document.getElementById('revoke-status').style.display = 'none';
    document.getElementById('btn-confirm-revoke').disabled = false;
    document.getElementById('btn-confirm-revoke').textContent = 'Revoke Identity';
    document.getElementById('revoke-modal').style.display = 'flex';
}

function closeRevokeModal() {
    document.getElementById('revoke-modal').style.display = 'none';
    revokeChildID = null;
}

async function confirmRevoke() {
    const password = document.getElementById('revoke-password').value;
    const statusEl = document.getElementById('revoke-status');
    const btn = document.getElementById('btn-confirm-revoke');

    if (!password) {
        statusEl.textContent = 'Please enter your password to confirm.';
        statusEl.className = 'revoke-status error';
        statusEl.style.display = 'block';
        return;
    }

    btn.disabled = true;
    btn.textContent = 'Revoking...';
    statusEl.textContent = 'Sending revocation to blockchain...';
    statusEl.className = 'revoke-status pending';
    statusEl.style.display = 'block';

    try {
        const res = await fetch('/api/parent/child/revoke', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ child_id: revokeChildID, password })
        });

        const data = await res.json();
        if (!res.ok) {
            statusEl.textContent = data.error || 'Revocation failed.';
            statusEl.className = 'revoke-status error';
            btn.disabled = false;
            btn.textContent = 'Revoke Identity';
            return;
        }

        statusEl.textContent = 'Identity revoked permanently.';
        statusEl.className = 'revoke-status success';
        btn.textContent = 'Done';

        setTimeout(() => {
            closeRevokeModal();
            refreshDashboard();
        }, 1500);

    } catch (err) {
        statusEl.textContent = 'Network error.';
        statusEl.className = 'revoke-status error';
        btn.disabled = false;
        btn.textContent = 'Revoke Identity';
    }
}

// ── Logout ───────────────────────────────────────────────────
async function logout() {
    await fetch('/api/parent/logout', { method: 'POST' });
    location.reload();
}

// ── Close modal on escape / overlay click ────────────────────
document.addEventListener('keydown', e => {
    if (e.key === 'Escape') closeRevokeModal();
});
document.getElementById('revoke-modal')?.addEventListener('click', e => {
    if (e.target.classList.contains('modal-overlay')) closeRevokeModal();
});

// ── Auto-check session on load ───────────────────────────────
window.addEventListener('load', async () => {
    try {
        const res = await fetch('/api/parent/me');
        if (res.ok) {
            const data = await res.json();
            await onLoginSuccess(data.email || data.phone || 'Parent');
        }
    } catch (e) {
        // Not logged in — show auth screen (already visible by default)
    }
});
