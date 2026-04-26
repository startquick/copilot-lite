#!/bin/bash

API_BASE="${GROKPI_ADMIN_BASE_URL:-http://127.0.0.1:8080}"
COOKIE_JAR="$(mktemp)"

cleanup() {
    rm -f "$COOKIE_JAR"
}
trap cleanup EXIT

admin_curl() {
    curl -s -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
      -H "Authorization: Bearer $APP_KEY" \
      "$@"
}

json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

echo "============================"
echo " GrokPi Admin Utility (.sh) "
echo "============================"

APP_KEY=$1
if [ -z "$APP_KEY" ]; then
    read -s -p "Enter App Key (Admin Password): " APP_KEY
    echo ""
fi

LOGIN_PAYLOAD="{\"key\":\"$(json_escape "$APP_KEY")\"}"
LOGIN_CODE=$(curl -s -o "$COOKIE_JAR.body" -w "%{http_code}" -c "$COOKIE_JAR" \
  -X POST "$API_BASE/admin/login" \
  -H "Content-Type: application/json" \
  -d "$LOGIN_PAYLOAD")

if [ "$LOGIN_CODE" != "200" ]; then
    echo -e "\e[31mLogin failed.\e[0m"
    cat "$COOKIE_JAR.body"
    rm -f "$COOKIE_JAR.body"
    exit 1
fi
rm -f "$COOKIE_JAR.body"
echo -e "\e[32mLogin successful!\e[0m"

while true; do
    echo ""
    echo "--- Upstream Token Management ---"
    echo "1. Add Upstream Grok Token(s)"
    echo "2. List Upstream Tokens"
    echo "3. Delete Upstream Token"
    echo ""
    echo "--- Client API Key Management ---"
    echo "4. Create a new Client API Key"
    echo "5. List Client API Keys"
    echo "6. Delete a Client API Key"
    echo ""
    echo "7. Exit"
    read -p "Choice [1-7]: " choice

    if [ "$choice" = "1" ]; then
        read -p "Enter tokens (comma separated for multiple): " upTokens
        JSON_TOKENS=$(echo "$upTokens" | tr ',' '\n' | sed 's/^[ \t]*//;s/[ \t]*$//' | sed 's/.*/"&"/' | paste -sd, -)
        if [ ! -z "$JSON_TOKENS" ]; then
            JSON_PAYLOAD="{\"operation\": \"import\", \"tokens\": [$JSON_TOKENS]}"
            K_RES=$(admin_curl -X POST "$API_BASE/admin/tokens/batch" \
              -H "Content-Type: application/json" \
              -d "$JSON_PAYLOAD")
            FAILED=$(echo "$K_RES" | grep -o '"failed":[0-9]*' | cut -d: -f2 | head -n1)
            
            if [ "$FAILED" = "0" ] || [ -z "$FAILED" ]; then
                echo -e "\e[32mTokens added successfully!\e[0m"
            else
                echo -e "\e[31mFailed to add tokens. Details: $K_RES\e[0m"
            fi
        fi
    elif [ "$choice" = "2" ]; then
        RES=$(admin_curl -X GET "$API_BASE/admin/tokens?page_size=100")
        if command -v python3 &>/dev/null; then
            echo -e "\e[36m\n--- Upstream Token List ---\e[0m"
            echo "$RES" | python3 -c '
import sys, json
try:
  data = json.load(sys.stdin)
  if "error" in data:
      print("API Error: " + str(data.get("error")))
  else:
      print("{:<5} | {:<8} | {:<11} | {:<4} | {:<5} | {}".format("ID", "STATUS", "POOL", "PRIO", "QUOTA", "TOKEN"))
      print("-" * 80)
      for t in data.get("data", []):
          print("{:<5} | {:<8} | {:<11} | {:<4} | {:<5} | {}".format(
              t.get("id", ""), 
              t.get("status", ""), 
              str(t.get("pool", ""))[:11], 
              t.get("priority", 0), 
              t.get("chat_quota", ""), 
              t.get("token", "")
          ))
except Exception as e:
  print("Failed to parse JSON")'
        else
            echo "$RES"
        fi
    elif [ "$choice" = "3" ]; then
        read -p "Enter the Token ID to delete (e.g. 1): " delId
        if [ ! -z "$delId" ]; then
            DEL_CODE=$(admin_curl -o /dev/null -w "%{http_code}" -X DELETE "$API_BASE/admin/tokens/$delId")
            if [ "$DEL_CODE" = "204" ] || [ "$DEL_CODE" = "200" ]; then
                echo -e "\e[32mSuccessfully deleted Token ID: $delId\e[0m"
            else
                echo -e "\e[31mFailed to delete Token ID: $delId\e[0m"
            fi
        fi
    elif [ "$choice" = "4" ]; then
        read -p "Enter an alias/name for the new API Key: " keyName
        if [ -z "$keyName" ]; then keyName="UnnamedKey"; fi
        K_RES=$(admin_curl -X POST "$API_BASE/admin/apikeys" \
          -H "Content-Type: application/json" \
          -d "{\"name\": \"$keyName\", \"limit_type\": \"unlimited\"}")
        API_KEY=$(echo "$K_RES" | grep -o '"key":"[^"]*' | grep -o '[^"]*$')
        echo -e "\e[32mSuccessfully created API Key: $API_KEY\e[0m"
    elif [ "$choice" = "5" ]; then
        RES=$(admin_curl -X GET "$API_BASE/admin/apikeys?page_size=100")
        if command -v python3 &>/dev/null; then
            echo -e "\e[36m\n--- Client API Key List ---\e[0m"
            echo "$RES" | python3 -c '
import sys, json
try:
  data = json.load(sys.stdin)
  print("{:<5} | {:<10} | {:<15} | {}".format("ID", "STATUS", "NAME", "KEY"))
  print("-" * 75)
  for t in data.get("data", []):
      print("{:<5} | {:<10} | {:<15} | {}".format(t.get("id",""), t.get("status",""), str(t.get("name",""))[:15], t.get("key","")))
except Exception:
  print("Failed to parse JSON")'
        else
            echo "$RES"
        fi
    elif [ "$choice" = "6" ]; then
        read -p "Enter the API Key ID to delete: " delId
        if [ ! -z "$delId" ]; then
            DEL_CODE=$(admin_curl -o /dev/null -w "%{http_code}" -X DELETE "$API_BASE/admin/apikeys/$delId")
            if [ "$DEL_CODE" = "204" ] || [ "$DEL_CODE" = "200" ]; then
                echo -e "\e[32mSuccessfully deleted API Key ID: $delId\e[0m"
            else
                echo -e "\e[31mFailed to delete API Key ID: $delId\e[0m"
            fi
        fi
    elif [ "$choice" = "7" ]; then
        break
    fi
done
