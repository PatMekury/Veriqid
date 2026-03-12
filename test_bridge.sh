#!/bin/bash
# ============================================================
# Veriqid Bridge API — End-to-End Test Script
# ============================================================
#
# Usage: ./test_bridge.sh <GANACHE_PRIVATE_KEY_WITHOUT_0x>
#
# Requires:
#   - Ganache running on port 7545
#   - Veriqid contract deployed (truffle migrate --reset)
#   - Bridge running on port 9090 with the Veriqid contract address
#
# Tests 7 groups with 17 tests:
#   1. Status endpoint
#   2. Challenge endpoint
#   3. Identity creation (with age_bracket)
#   4. Registration proof
#   5. Auth proof
#   6. Error handling
#   7. CORS preflight
# ============================================================

set -euo pipefail

BRIDGE="http://localhost:9090"
ETHKEY="${1:-}"
PASS=0
FAIL=0
TOTAL=0

if [ -z "$ETHKEY" ]; then
    echo "Usage: ./test_bridge.sh <GANACHE_PRIVATE_KEY_WITHOUT_0x>"
    echo ""
    echo "Get a private key from Ganache output (remove the 0x prefix)."
    exit 1
fi

# Clean up any leftover test keys from previous runs
rm -f ./bridge_test_key1 ./bridge_test_key2

# Helper: check a condition and print PASS/FAIL
check() {
    local description="$1"
    local condition="$2"
    TOTAL=$((TOTAL + 1))
    if eval "$condition"; then
        echo "  PASS $description"
        PASS=$((PASS + 1))
    else
        echo "  FAIL $description"
        FAIL=$((FAIL + 1))
    fi
}

echo ""
echo "==========================================="
echo "  Veriqid Bridge API — Test Suite"
echo "==========================================="
echo ""

# ─── Test 1: Status ──────────────────────────────────────────
echo "=== Test 1: GET /api/status ==="

STATUS=$(curl -s "$BRIDGE/api/status")
STATUS_OK=$(echo "$STATUS" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status',''))" 2>/dev/null)
VERSION=$(echo "$STATUS" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('version',''))" 2>/dev/null)

check "Status endpoint (status=ok)" '[ "$STATUS_OK" = "ok" ]'
check "Version present (version=$VERSION)" '[ -n "$VERSION" ]'

echo ""

# ─── Test 2: Challenge ───────────────────────────────────────
echo "=== Test 2: GET /api/identity/challenge ==="

CHAL_RESP=$(curl -s "$BRIDGE/api/identity/challenge")
CHALLENGE=$(echo "$CHAL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('challenge',''))" 2>/dev/null)
CHAL_LEN=${#CHALLENGE}

check "Challenge returns hex string" '[ -n "$CHALLENGE" ]'
check "Challenge is 64 chars (32 bytes hex)" '[ "$CHAL_LEN" -eq 64 ]'

echo ""

# ─── Test 3: Create Identities ───────────────────────────────
echo "=== Test 3: POST /api/identity/create ==="

CREATE1=$(curl -s -X POST "$BRIDGE/api/identity/create" \
    -H "Content-Type: application/json" \
    -d "{
        \"keypath\": \"./bridge_test_key1\",
        \"ethkey\": \"$ETHKEY\",
        \"age_bracket\": 1
    }")
SUCCESS1=$(echo "$CREATE1" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', False))" 2>/dev/null)
MPK1=$(echo "$CREATE1" | python3 -c "import sys,json; print(json.load(sys.stdin).get('mpk_hex',''))" 2>/dev/null)

check "Create identity 1 (success=$SUCCESS1)" '[ "$SUCCESS1" = "True" ]'
check "MPK hex returned (len=${#MPK1})" '[ ${#MPK1} -eq 66 ]'

# Verify key file is 32 bytes
if [ -f "./bridge_test_key1" ]; then
    KEY1_SIZE=$(wc -c < ./bridge_test_key1)
    check "Key file is 32 bytes (size=$KEY1_SIZE)" '[ "$KEY1_SIZE" -eq 32 ]'
else
    check "Key file exists" 'false'
fi

CREATE2=$(curl -s -X POST "$BRIDGE/api/identity/create" \
    -H "Content-Type: application/json" \
    -d "{
        \"keypath\": \"./bridge_test_key2\",
        \"ethkey\": \"$ETHKEY\",
        \"age_bracket\": 2
    }")
SUCCESS2=$(echo "$CREATE2" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', False))" 2>/dev/null)

check "Create identity 2 with age_bracket=2 (success=$SUCCESS2)" '[ "$SUCCESS2" = "True" ]'

echo ""

# ─── Test 4: Registration Proof ──────────────────────────────
echo "=== Test 4: POST /api/identity/register ==="

# Get a fresh challenge
REG_CHALLENGE=$(curl -s "$BRIDGE/api/identity/challenge" | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])" 2>/dev/null)

# SHA-256 hash of "test_service" as the service name
SERVICE_NAME=$(echo -n "test_service" | sha256sum | cut -d' ' -f1)

REG_RESP=$(curl -s -X POST "$BRIDGE/api/identity/register" \
    -H "Content-Type: application/json" \
    -d "{
        \"keypath\": \"./bridge_test_key1\",
        \"service_name\": \"$SERVICE_NAME\",
        \"challenge\": \"$REG_CHALLENGE\"
    }")
REG_SUCCESS=$(echo "$REG_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', False))" 2>/dev/null)
PROOF_HEX=$(echo "$REG_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('proof_hex',''))" 2>/dev/null)
SPK_HEX=$(echo "$REG_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('spk_hex',''))" 2>/dev/null)
RING_SIZE=$(echo "$REG_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('ring_size',0))" 2>/dev/null)

check "Registration proof generated (success=$REG_SUCCESS)" '[ "$REG_SUCCESS" = "True" ]'
check "Proof hex present (len=${#PROOF_HEX})" '[ ${#PROOF_HEX} -gt 0 ]'
check "SPK hex present (len=${#SPK_HEX})" '[ ${#SPK_HEX} -eq 66 ]'
check "Ring size >= 2 (ring_size=$RING_SIZE)" '[ "$RING_SIZE" -ge 2 ]'

echo ""

# ─── Test 5: Auth Proof ──────────────────────────────────────
echo "=== Test 5: POST /api/identity/auth ==="

AUTH_CHALLENGE=$(curl -s "$BRIDGE/api/identity/challenge" | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])" 2>/dev/null)

AUTH_RESP=$(curl -s -X POST "$BRIDGE/api/identity/auth" \
    -H "Content-Type: application/json" \
    -d "{
        \"keypath\": \"./bridge_test_key1\",
        \"service_name\": \"$SERVICE_NAME\",
        \"challenge\": \"$AUTH_CHALLENGE\"
    }")
AUTH_SUCCESS=$(echo "$AUTH_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', False))" 2>/dev/null)
AUTH_PROOF=$(echo "$AUTH_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('auth_proof_hex',''))" 2>/dev/null)
AUTH_LEN=${#AUTH_PROOF}

check "Auth proof generated (success=$AUTH_SUCCESS)" '[ "$AUTH_SUCCESS" = "True" ]'
check "Auth proof is 130 chars / 65 bytes (len=$AUTH_LEN)" '[ "$AUTH_LEN" -eq 130 ]'

echo ""

# ─── Test 6: Error Handling ──────────────────────────────────
echo "=== Test 6: Error handling ==="

# GET on a POST endpoint
ERR1=$(curl -s "$BRIDGE/api/identity/create")
ERR1_SUCCESS=$(echo "$ERR1" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', True))" 2>/dev/null)
check "GET on POST endpoint (success=$ERR1_SUCCESS)" '[ "$ERR1_SUCCESS" = "False" ]'

# Missing ethkey
ERR2=$(curl -s -X POST "$BRIDGE/api/identity/create" \
    -H "Content-Type: application/json" \
    -d '{"keypath": "./err_key"}')
ERR2_SUCCESS=$(echo "$ERR2" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', True))" 2>/dev/null)
check "Missing ethkey (success=$ERR2_SUCCESS)" '[ "$ERR2_SUCCESS" = "False" ]'

# Invalid JSON
ERR3=$(curl -s -X POST "$BRIDGE/api/identity/create" \
    -H "Content-Type: application/json" \
    -d '{bad json}')
ERR3_SUCCESS=$(echo "$ERR3" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', True))" 2>/dev/null)
check "Invalid JSON (success=$ERR3_SUCCESS)" '[ "$ERR3_SUCCESS" = "False" ]'

# Nonexistent key file for auth
ERR4=$(curl -s -X POST "$BRIDGE/api/identity/auth" \
    -H "Content-Type: application/json" \
    -d "{
        \"keypath\": \"./nonexistent_key_9999\",
        \"service_name\": \"$SERVICE_NAME\",
        \"challenge\": \"$AUTH_CHALLENGE\"
    }")
ERR4_SUCCESS=$(echo "$ERR4" | python3 -c "import sys,json; print(json.load(sys.stdin).get('success', True))" 2>/dev/null)
check "Nonexistent key (success=$ERR4_SUCCESS)" '[ "$ERR4_SUCCESS" = "False" ]'

echo ""

# ─── Test 7: CORS ────────────────────────────────────────────
echo "=== Test 7: CORS preflight ==="

CORS_HEADERS=$(curl -s -I -X OPTIONS "$BRIDGE/api/status" 2>/dev/null)
CORS_ORIGIN=$(echo "$CORS_HEADERS" | grep -i "Access-Control-Allow-Origin" | head -1)
CORS_STATUS=$(echo "$CORS_HEADERS" | head -1 | grep -o "[0-9][0-9][0-9]")

check "OPTIONS returns 204 (status=$CORS_STATUS)" '[ "$CORS_STATUS" = "204" ]'
check "CORS Allow-Origin header present" 'echo "$CORS_ORIGIN" | grep -q "*"'

echo ""

# ─── Cleanup ─────────────────────────────────────────────────
rm -f ./bridge_test_key1 ./bridge_test_key2

# ─── Summary ─────────────────────────────────────────────────
echo "==========================================="
if [ "$FAIL" -eq 0 ]; then
    echo "  ALL $TOTAL TESTS PASSED"
else
    echo "  $PASS/$TOTAL PASSED, $FAIL FAILED"
fi
echo "==========================================="
echo ""

exit "$FAIL"
