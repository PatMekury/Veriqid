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
    pending:  { label: 'Pending', class: 'pending',  icon: '⏳' },
    verified: { label: 'Active',  class: 'active',   icon: '✓' },
    revoked:  { label: 'Revoked', class: 'revoked',  icon: '✗' }
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

        // Move to verification step
        document.getElementById('verify-child-name').textContent = name;
        document.getElementById('onboard-step1').style.display = 'none';
        document.getElementById('onboard-step2').style.display = 'block';

    } catch (err) {
        alert('Network error.');
    }
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
            btn.textContent = '✓ Simulate Verification Approval';
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
        btn.textContent = '✓ Simulate Verification Approval';
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
            actionBtn = `<div class="btn-revoked">✗ Revoked</div>`;
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
            return `
                <div class="activity-item">
                    <div class="activity-icon ${isRevoke ? 'revoked' : 'registered'}">
                        ${isRevoke ? '✗' : '✓'}
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
