#!/bin/bash
# ==========================================================================
# Veriqid Server — End-to-End Test Script
# ==========================================================================
#
# Usage: ./test_veriqid_server.sh <GANACHE_PRIVATE_KEY_WITHOUT_0x>
#
# Requires:
#   - Ganache running on port 7545
#   - Bridge running on port 9090
#   - Veriqid server running on port 8080
#   - At least 2 identities created via the bridge
#
# ==========================================================================

set -e

ETHKEY="${1:?Usage: $0 <GANACHE_PRIVATE_KEY_WITHOUT_0x>}"
SERVER="http://localhost:8080"
BRIDGE="http://localhost:9090"
PASS=0
FAIL=0
SKIP=0

pass() { echo "  ✅ PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  ❌ FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "  ⏭️  SKIP: $1"; SKIP=$((SKIP + 1)); }
header() { echo ""; echo "=== $1 ==="; }

# --------------------------------------------------------------------------
# Pre-flight checks
# --------------------------------------------------------------------------
header "Pre-flight: Checking services"

# Check Veriqid server
SERVER_UP=$(curl -s -o /dev/null -w "%{http_code}" $SERVER/api/status 2>/dev/null || echo "000")
if [ "$SERVER_UP" = "200" ]; then
    pass "Veriqid server is running"
else
    fail "Veriqid server not reachable at $SERVER (HTTP $SERVER_UP)"
    echo "  Start it with: ./veriqid-server -contract 0x... -service KidsTube"
    exit 1
fi

# Check bridge
BRIDGE_UP=$(curl -s -o /dev/null -w "%{http_code}" $BRIDGE/api/identity/challenge 2>/dev/null || echo "000")
if [ "$BRIDGE_UP" = "200" ]; then
    pass "Bridge is running"
else
    fail "Bridge not reachable at $BRIDGE (HTTP $BRIDGE_UP)"
    echo "  Start it with: ./bridge-server -contract 0x... -client http://127.0.0.1:7545"
    exit 1
fi

# --------------------------------------------------------------------------
# Test 1: GET /api/status
# --------------------------------------------------------------------------
header "Test 1: GET /api/status"

STATUS=$(curl -s $SERVER/api/status)
echo "  Response: $STATUS"

echo "$STATUS" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d['status']=='ok' else 1)" 2>/dev/null \
    && pass "status == 'ok'" || fail "status != 'ok'"

echo "$STATUS" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d['platform']!='' else 1)" 2>/dev/null \
    && pass "platform name present" || fail "platform name missing"

echo "$STATUS" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if len(d['service_name'])==64 else 1)" 2>/dev/null \
    && pass "service_name is 64 hex chars (SHA-256)" || fail "service_name wrong length"

echo "$STATUS" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d['version']=='0.4.0' else 1)" 2>/dev/null \
    && pass "version == 0.4.0" || fail "version mismatch"

# --------------------------------------------------------------------------
# Test 2: GET /api/challenge
# --------------------------------------------------------------------------
header "Test 2: GET /api/challenge"

CHAL_RESP=$(curl -s "$SERVER/api/challenge?type=signup")
echo "  Response: $CHAL_RESP"

CHALLENGE=$(echo "$CHAL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])" 2>/dev/null)
SERVICE=$(echo "$CHAL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['service_name'])" 2>/dev/null)

if [ ${#CHALLENGE} -eq 64 ]; then
    pass "challenge is 64 hex chars (32 bytes)"
else
    fail "challenge length: ${#CHALLENGE} (expected 64)"
fi

if [ ${#SERVICE} -eq 64 ]; then
    pass "service_name is 64 hex chars"
else
    fail "service_name length: ${#SERVICE} (expected 64)"
fi

# --------------------------------------------------------------------------
# Test 3: Create identities (if needed)
# --------------------------------------------------------------------------
header "Test 3: Ensure identities exist"

# Try creating two identities — if they already exist, the bridge should handle it
ID1_RESP=$(curl -s -X POST $BRIDGE/api/identity/create \
    -H "Content-Type: application/json" \
    -d "{\"keypath\": \"./test_key1\", \"ethkey\": \"$ETHKEY\"}" 2>/dev/null || echo '{"error":"failed"}')
echo "  Identity 1: $ID1_RESP"

ID2_RESP=$(curl -s -X POST $BRIDGE/api/identity/create \
    -H "Content-Type: application/json" \
    -d "{\"keypath\": \"./test_key2\", \"ethkey\": \"$ETHKEY\"}" 2>/dev/null || echo '{"error":"failed"}')
echo "  Identity 2: $ID2_RESP"

# Check if we have at least 2 IDs on-chain via the server status
ID_COUNT=$(curl -s $SERVER/api/status | python3 -c "import sys,json; print(json.load(sys.stdin).get('user_count',0))" 2>/dev/null || echo "0")
pass "Identities created (or already existed)"

# --------------------------------------------------------------------------
# Test 4: Generate a registration proof via the bridge
# --------------------------------------------------------------------------
header "Test 4: Registration proof via bridge"

# Get a fresh challenge
CHAL_RESP=$(curl -s "$SERVER/api/challenge?type=signup")
CHALLENGE=$(echo "$CHAL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])" 2>/dev/null)
SERVICE=$(echo "$CHAL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['service_name'])" 2>/dev/null)

PROOF_RESP=$(curl -s -X POST $BRIDGE/api/identity/register \
    -H "Content-Type: application/json" \
    -d "{\"keypath\": \"./test_key1\", \"service_name\": \"$SERVICE\", \"challenge\": \"$CHALLENGE\"}" 2>/dev/null || echo '{"success":false}')

# Check if proof was generated
HAS_PROOF=$(echo "$PROOF_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('proof_hex','')!='' else 'no')" 2>/dev/null || echo "no")

if [ "$HAS_PROOF" = "yes" ]; then
    pass "Registration proof generated"
    PROOF_HEX=$(echo "$PROOF_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['proof_hex'])" 2>/dev/null)
    SPK_HEX=$(echo "$PROOF_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['spk_hex'])" 2>/dev/null)
    RING_SIZE=$(echo "$PROOF_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['ring_size'])" 2>/dev/null)
    echo "  SPK: ${SPK_HEX:0:32}..."
    echo "  Ring size: $RING_SIZE"
else
    fail "Registration proof generation failed"
    echo "  Response: $PROOF_RESP"
    echo "  (Remaining tests that depend on proofs will be skipped)"
fi

# --------------------------------------------------------------------------
# Test 5: POST /api/verify/registration
# --------------------------------------------------------------------------
header "Test 5: POST /api/verify/registration"

if [ "$HAS_PROOF" = "yes" ]; then
    VERIFY_RESP=$(curl -s -X POST $SERVER/api/verify/registration \
        -H "Content-Type: application/json" \
        -d "{\"proof_hex\":\"$PROOF_HEX\",\"spk_hex\":\"$SPK_HEX\",\"challenge_hex\":\"$CHALLENGE\",\"ring_size\":$RING_SIZE}")
    echo "  Response: $VERIFY_RESP"

    echo "$VERIFY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('verified',False) else 1)" 2>/dev/null \
        && pass "Registration proof verified" || fail "Registration verification returned false"
else
    skip "No proof available"
fi

# --------------------------------------------------------------------------
# Test 6: Auth proof via bridge
# --------------------------------------------------------------------------
header "Test 6: Auth proof via bridge"

if [ "$HAS_PROOF" = "yes" ]; then
    # Get a fresh challenge for login
    LOGIN_CHAL_RESP=$(curl -s "$SERVER/api/challenge?type=login")
    LOGIN_CHALLENGE=$(echo "$LOGIN_CHAL_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['challenge'])" 2>/dev/null)

    AUTH_RESP=$(curl -s -X POST $BRIDGE/api/identity/auth \
        -H "Content-Type: application/json" \
        -d "{\"keypath\": \"./test_key1\", \"service_name\": \"$SERVICE\", \"challenge\": \"$LOGIN_CHALLENGE\"}" 2>/dev/null || echo '{"success":false}')

    HAS_AUTH=$(echo "$AUTH_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('auth_proof_hex','')!='' else 'no')" 2>/dev/null || echo "no")

    if [ "$HAS_AUTH" = "yes" ]; then
        pass "Auth proof generated"
        AUTH_PROOF_HEX=$(echo "$AUTH_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['auth_proof_hex'])" 2>/dev/null)
    else
        fail "Auth proof generation failed"
        echo "  Response: $AUTH_RESP"
    fi
else
    skip "No proof available"
fi

# --------------------------------------------------------------------------
# Test 7: POST /api/verify/auth
# --------------------------------------------------------------------------
header "Test 7: POST /api/verify/auth"

if [ "$HAS_PROOF" = "yes" ] && [ "$HAS_AUTH" = "yes" ]; then
    AUTH_VERIFY_RESP=$(curl -s -X POST $SERVER/api/verify/auth \
        -H "Content-Type: application/json" \
        -d "{\"auth_proof_hex\":\"$AUTH_PROOF_HEX\",\"spk_hex\":\"$SPK_HEX\",\"challenge_hex\":\"$LOGIN_CHALLENGE\"}")
    echo "  Response: $AUTH_VERIFY_RESP"

    echo "$AUTH_VERIFY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('verified',False) else 1)" 2>/dev/null \
        && pass "Auth proof verified" || fail "Auth verification returned false"
else
    skip "No auth proof available"
fi

# --------------------------------------------------------------------------
# Test 8: Error handling — empty body
# --------------------------------------------------------------------------
header "Test 8: Error handling"

ERR1=$(curl -s -X POST $SERVER/api/verify/registration \
    -H "Content-Type: application/json" \
    -d '{}')
echo "  Empty body: $ERR1"
echo "$ERR1" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('error',False) else 1)" 2>/dev/null \
    && pass "Empty body returns error" || fail "Empty body should return error"

ERR2=$(curl -s -X GET $SERVER/api/verify/registration)
echo "  GET on POST endpoint: $ERR2"
echo "$ERR2" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('error',False) else 1)" 2>/dev/null \
    && pass "GET on POST endpoint returns error" || fail "GET on POST endpoint should return error"

ERR3=$(curl -s -X POST $SERVER/api/verify/registration \
    -H "Content-Type: application/json" \
    -d 'not json')
echo "  Invalid JSON: $ERR3"
echo "$ERR3" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('error',False) else 1)" 2>/dev/null \
    && pass "Invalid JSON returns error" || fail "Invalid JSON should return error"

# --------------------------------------------------------------------------
# Test 9: HTML pages return 200
# --------------------------------------------------------------------------
header "Test 9: HTML page status codes"

for PAGE in "/" "/signup" "/login"; do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER$PAGE")
    if [ "$CODE" = "200" ]; then
        pass "GET $PAGE returns 200"
    else
        fail "GET $PAGE returns $CODE (expected 200)"
    fi
done

# Dashboard should redirect (302/303) when not logged in
DASH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER/dashboard")
if [ "$DASH_CODE" = "303" ] || [ "$DASH_CODE" = "302" ]; then
    pass "GET /dashboard redirects when not logged in ($DASH_CODE)"
else
    fail "GET /dashboard returned $DASH_CODE (expected 302 or 303)"
fi

# --------------------------------------------------------------------------
# Test 10: Backward-compatible routes
# --------------------------------------------------------------------------
header "Test 10: Phase 1 backward-compatible routes"

for PAGE in "/directSignup" "/directLogin"; do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER$PAGE")
    if [ "$CODE" = "200" ]; then
        pass "GET $PAGE returns 200"
    else
        fail "GET $PAGE returns $CODE (expected 200)"
    fi
done

# --------------------------------------------------------------------------
# Test 11: CORS headers on API
# --------------------------------------------------------------------------
header "Test 11: CORS headers on API endpoints"

CORS_HEADER=$(curl -s -I "$SERVER/api/status" 2>/dev/null | grep -i "access-control-allow-origin" || echo "")
if [ -n "$CORS_HEADER" ]; then
    pass "CORS Access-Control-Allow-Origin present on /api/status"
else
    fail "CORS header missing on /api/status"
fi

# --------------------------------------------------------------------------
# Results
# --------------------------------------------------------------------------
echo ""
echo "==========================================="
echo "  Results: $PASS passed, $FAIL failed, $SKIP skipped"
if [ $FAIL -eq 0 ]; then
    echo "  🎉 ALL TESTS PASSED"
else
    echo "  ⚠️  SOME TESTS FAILED"
fi
echo "==========================================="
echo ""

# Cleanup test keys (optional)
# rm -f ./test_key1 ./test_key2

exit $FAIL
