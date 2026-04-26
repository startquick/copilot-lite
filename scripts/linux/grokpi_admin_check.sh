#!/bin/bash

API_BASE="${GROKPI_ADMIN_BASE_URL:-http://127.0.0.1:8080}"
COOKIE_JAR="$(mktemp)"

cleanup() {
    rm -f "$COOKIE_JAR" "$COOKIE_JAR.body"
}
trap cleanup EXIT

json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

APP_KEY="$1"
if [ -z "$APP_KEY" ]; then
    read -s -p "Enter App Key (Admin Password): " APP_KEY
    echo ""
fi

if [ -z "$APP_KEY" ]; then
    echo "APP_KEY is required"
    exit 1
fi

LOGIN_PAYLOAD="{\"key\":\"$(json_escape "$APP_KEY")\"}"
LOGIN_CODE=$(curl -s -o "$COOKIE_JAR.body" -w "%{http_code}" -c "$COOKIE_JAR" \
  -X POST "$API_BASE/admin/login" \
  -H "Content-Type: application/json" \
  -d "$LOGIN_PAYLOAD")

echo "== POST /admin/login =="
if [ "$LOGIN_CODE" != "200" ]; then
    echo "login failed (HTTP $LOGIN_CODE)"
    cat "$COOKIE_JAR.body"
    exit 1
fi
cat "$COOKIE_JAR.body"
echo ""

VERIFY_CODE=$(curl -s -o "$COOKIE_JAR.body" -w "%{http_code}" -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -X GET "$API_BASE/admin/verify" \
  -H "Authorization: Bearer $APP_KEY")

echo "== GET /admin/verify =="
cat "$COOKIE_JAR.body"
echo ""

if [ "$VERIFY_CODE" = "200" ]; then
    echo "admin auth check: ok"
    exit 0
fi

echo "admin auth check: failed (HTTP $VERIFY_CODE)"
exit 1
