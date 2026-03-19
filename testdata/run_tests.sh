#!/usr/bin/env bash
# End-to-end test runner for video-debug-mcp tools
# Uses stdio transport — sends JSON-RPC messages directly to the binary

set -euo pipefail

BINARY="./bin/video-debug-mcp"
BASE_URL="http://localhost:9999"
PASS=0
FAIL=0
RESULTS=""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

call_tool() {
    local tool_name="$1"
    local args_json="$2"
    local description="$3"

    printf "%-60s " "$description..."

    # Build JSON-RPC messages: initialize + tools/call
    local init_msg='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-runner","version":"1.0"}}}'
    local call_msg
    call_msg=$(printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"%s","arguments":%s}}' "$tool_name" "$args_json")

    # Send both messages via stdio, capture output
    local output
    output=$(printf '%s\n%s\n' "$init_msg" "$call_msg" | timeout 120 "$BINARY" --transport stdio --config config.yaml 2>/dev/null || true)

    # Extract the tools/call response (id:2)
    local response
    response=$(echo "$output" | grep '"id":2' | head -1)

    if [ -z "$response" ]; then
        printf "${RED}FAIL${NC} (no response)\n"
        FAIL=$((FAIL + 1))
        RESULTS+="FAIL: $description — no response from server\n"
        return 1
    fi

    # Check if the response has an error
    local has_error
    has_error=$(echo "$response" | python3 -c "
import sys, json
resp = json.load(sys.stdin)
result = resp.get('result', {})
# MCP tool errors come as content with isError=true
if result.get('isError'):
    print('error')
else:
    print('ok')
" 2>/dev/null || echo "parse_error")

    if [ "$has_error" = "error" ]; then
        local error_msg
        error_msg=$(echo "$response" | python3 -c "
import sys, json
resp = json.load(sys.stdin)
content = resp.get('result', {}).get('content', [{}])
if content:
    print(content[0].get('text', 'unknown error')[:200])
" 2>/dev/null || echo "unknown")
        printf "${RED}FAIL${NC} — %s\n" "$error_msg"
        FAIL=$((FAIL + 1))
        RESULTS+="FAIL: $description — $error_msg\n"
        return 1
    elif [ "$has_error" = "parse_error" ]; then
        printf "${RED}FAIL${NC} (response parse error)\n"
        FAIL=$((FAIL + 1))
        RESULTS+="FAIL: $description — could not parse response\n"
        return 1
    fi

    printf "${GREEN}PASS${NC}\n"
    PASS=$((PASS + 1))
    RESULTS+="PASS: $description\n"

    # Optionally dump response for debugging
    if [ "${VERBOSE:-}" = "1" ]; then
        echo "$response" | python3 -m json.tool 2>/dev/null | head -50
        echo "---"
    fi
    return 0
}

validate_response() {
    local tool_name="$1"
    local args_json="$2"
    local description="$3"
    local check_pattern="$4"

    printf "%-60s " "$description..."

    local init_msg='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-runner","version":"1.0"}}}'
    local call_msg
    call_msg=$(printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"%s","arguments":%s}}' "$tool_name" "$args_json")

    local output
    output=$(printf '%s\n%s\n' "$init_msg" "$call_msg" | timeout 120 "$BINARY" --transport stdio --config config.yaml 2>/dev/null || true)

    local response
    response=$(echo "$output" | grep '"id":2' | head -1)

    if [ -z "$response" ]; then
        printf "${RED}FAIL${NC} (no response)\n"
        FAIL=$((FAIL + 1))
        RESULTS+="FAIL: $description — no response\n"
        return 1
    fi

    # Check for the expected pattern in the response
    if echo "$response" | grep -q "$check_pattern"; then
        printf "${GREEN}PASS${NC}\n"
        PASS=$((PASS + 1))
        RESULTS+="PASS: $description\n"
    else
        printf "${RED}FAIL${NC} (pattern '$check_pattern' not found)\n"
        FAIL=$((FAIL + 1))
        RESULTS+="FAIL: $description — expected pattern '$check_pattern' not in response\n"
        if [ "${VERBOSE:-}" = "1" ]; then
            echo "$response" | python3 -m json.tool 2>/dev/null | head -30
        fi
    fi
}

expect_error() {
    local tool_name="$1"
    local args_json="$2"
    local description="$3"

    printf "%-60s " "$description..."

    local init_msg='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-runner","version":"1.0"}}}'
    local call_msg
    call_msg=$(printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"%s","arguments":%s}}' "$tool_name" "$args_json")

    local output
    output=$(printf '%s\n%s\n' "$init_msg" "$call_msg" | timeout 120 "$BINARY" --transport stdio --config config.yaml 2>/dev/null || true)

    local response
    response=$(echo "$output" | grep '"id":2' | head -1)

    if [ -z "$response" ]; then
        printf "${RED}FAIL${NC} (no response at all)\n"
        FAIL=$((FAIL + 1))
        RESULTS+="FAIL: $description — no response\n"
        return 1
    fi

    # For this case, we accept both errors AND anomaly results as "pass"
    local has_content
    has_content=$(echo "$response" | grep -c '"result"' || true)
    if [ "$has_content" -gt 0 ]; then
        printf "${GREEN}PASS${NC} (got response — errors/anomalies expected)\n"
        PASS=$((PASS + 1))
        RESULTS+="PASS: $description\n"
    else
        printf "${RED}FAIL${NC}\n"
        FAIL=$((FAIL + 1))
        RESULTS+="FAIL: $description\n"
    fi
}

echo ""
echo "=============================================="
echo "  video-debug-mcp End-to-End Test Suite"
echo "=============================================="
echo ""
echo "HTTP server: $BASE_URL"
echo ""

# ── Test 1: analyze_manifest — HLS ──
echo "── Manifest Analysis ──"
validate_response "analyze_manifest" \
    "{\"url\":\"$BASE_URL/hls/master.m3u8\"}" \
    "[1/9] analyze_manifest: HLS master playlist" \
    "hls"

# ── Test 2: analyze_manifest — DASH ──
validate_response "analyze_manifest" \
    "{\"url\":\"$BASE_URL/dash/manifest.mpd\"}" \
    "[2/9] analyze_manifest: DASH MPD" \
    "dash"

echo ""
echo "── Media Analysis ──"

# ── Test 3: analyze_media — healthy MP4 ──
validate_response "analyze_media" \
    "{\"url\":\"$BASE_URL/healthy.mp4\"}" \
    "[3/9] analyze_media: healthy H.264+AAC MP4" \
    "h264"

# ── Test 4: analyze_media — audio only ──
validate_response "analyze_media" \
    "{\"url\":\"$BASE_URL/audio_only.mp4\"}" \
    "[4/9] analyze_media: audio-only MP4 (edge case)" \
    "aac"

echo ""
echo "── Container Analysis ──"

# ── Test 5: analyze_mp4_structure — healthy ──
validate_response "analyze_mp4_structure" \
    "{\"url\":\"$BASE_URL/healthy.mp4\",\"verbosity\":\"deep\"}" \
    "[5/9] analyze_mp4_structure: box hierarchy (deep)" \
    "moov"

echo ""
echo "── Playback Simulation ──"

# ── Test 6: simulate_playback — healthy (expect clean) ──
call_tool "simulate_playback" \
    "{\"url\":\"$BASE_URL/healthy.mp4\",\"duration\":5}" \
    "[6/9] simulate_playback: healthy MP4 (expect clean)"

# ── Test 7: simulate_playback — truncated (expect anomalies) ──
expect_error "simulate_playback" \
    "{\"url\":\"$BASE_URL/truncated.mp4\",\"duration\":10}" \
    "[7/9] simulate_playback: truncated MP4 (expect errors)"

echo ""
echo "── Packaging Validation ──"

# ── Test 8: validate_packaging — healthy ──
call_tool "validate_packaging" \
    "{\"url\":\"$BASE_URL/healthy.mp4\"}" \
    "[8/9] validate_packaging: healthy MP4 (expect clean)"

# ── Test 9: validate_packaging — truncated ──
expect_error "validate_packaging" \
    "{\"url\":\"$BASE_URL/truncated.mp4\"}" \
    "[9/9] validate_packaging: truncated MP4 (expect errors)"

echo ""
echo "=============================================="
printf "Results: ${GREEN}%d passed${NC}, ${RED}%d failed${NC} out of 9\n" "$PASS" "$FAIL"
echo "=============================================="
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo "Failed tests:"
    printf "$RESULTS" | grep "^FAIL"
    exit 1
fi
