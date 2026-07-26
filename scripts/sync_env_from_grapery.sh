#!/usr/bin/env bash
# 从 sibling grapery/.env 同步共享配置到 grapery-agent/.env（仅填空，不覆盖已有值）
# 用法：make sync-env-from-grapery
# 对齐 grapery/scripts/sync_apple_iap_env.sh：可选同步，不在 make run 时强制执行
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXAMPLE="${ROOT}/env.grapery-agent.dev.example"
ENV_FILE="${ROOT}/.env"
GRAPERY_ENV="${GRAPERY_ENV_FILE:-${ROOT}/../grapery/.env}"

cd "$ROOT"

if [ ! -f "$ENV_FILE" ]; then
  if [ ! -f "$EXAMPLE" ]; then
    echo "❌ missing $EXAMPLE"
    exit 1
  fi
  cp "$EXAMPLE" "$ENV_FILE"
  echo "✅ created $ENV_FILE from env.grapery-agent.dev.example"
fi

# Upsert KEY=VALUE into .env (replace existing KEY= line, or append)
upsert_env() {
  local key="$1" value="$2"
  [ -n "$value" ] || return 0
  python3 - "$ENV_FILE" "$key" "$value" <<'PY'
import re, sys
path, key, value = sys.argv[1], sys.argv[2], sys.argv[3]
text = open(path, encoding="utf-8").read()
line = f"{key}={value}"
pat = re.compile(rf"(?m)^{re.escape(key)}=.*$")
if pat.search(text):
    # only fill empty values; do not overwrite user-set secrets
    def repl(m):
        cur = m.group(0).split("=", 1)[1]
        if cur.strip() == "":
            return line
        return m.group(0)
    text = pat.sub(repl, text, count=1)
else:
    text = text.rstrip() + "\n" + line + "\n"
open(path, "w", encoding="utf-8").write(text)
PY
}

# Read KEY from a dotenv file (first match; ignore comments)
read_dotenv() {
  local file="$1" key="$2"
  [ -f "$file" ] || return 0
  python3 - "$file" "$key" <<'PY'
import re, sys
path, key = sys.argv[1], sys.argv[2]
text = open(path, encoding="utf-8").read()
m = re.search(rf"(?m)^{re.escape(key)}=(.*)$", text)
if not m:
    sys.exit(0)
val = m.group(1).strip().strip('"').strip("'")
print(val, end="")
PY
}

if [ -f "$GRAPERY_ENV" ]; then
  echo "🔗 syncing empty agent .env keys from $GRAPERY_ENV"

  # same-name shared keys
  for key in \
    HUOSHAN_API_KEY HUOSHAN_BASE_URL HUOSHAN_TEXT_MODEL HUOSHAN_IMAGE_MODEL HUOSHAN_VIDEO_MODEL \
    GEMINI_API_KEY GEMINI_BASE_URL \
    JWT_SECRET JWT_EXPIRY_HOURS \
    DB_DATABASE DB_USERNAME DB_PASSWORD DB_ADDRESS \
    REDIS_ADDRESS REDIS_PASSWORD REDIS_DATABASE \
    AGENT_TOKEN_SIGNING_KEY AGENT_TOKEN_REPLAY_CACHE_ENABLED AGENT_EXEC_FRAGMENT_PANEL_ENABLED \
    ALIYUN_API_KEY ALIYUN_SECRET_KEY ALIYUN_ENDPOINT ALIYUN_BUCKET ALIYUN_ROLE_ARN \
    ALIYUN_OSS_ACCESS_KEY_ID ALIYUN_OSS_ACCESS_KEY_SECRET ALIYUN_OSS_ROLE_ARN \
    ALIYUN_SMS_SIGN_NAME ALIYUN_SMS_TEMPLATE_CODE ALIYUN_SMS_REGION ALIYUN_SMS_ENDPOINT \
    ALIYUN_SMS_USE_DEFAULT_CREDENTIAL ALIYUN_SMS_ACCESS_KEY_ID ALIYUN_SMS_ACCESS_KEY_SECRET \
    ALIYUN_ACCESS_KEY_ID ALIYUN_ACCESS_KEY_SECRET
  do
    upsert_env "$key" "$(read_dotenv "$GRAPERY_ENV" "$key")"
  done

  # mapped keys: grapery → agent
  upsert_env "AGENT_TOKEN_VERIFY_KEY" "$(read_dotenv "$GRAPERY_ENV" "AGENT_TOKEN_SIGNING_KEY")"
  upsert_env "GRAPERY_API_KEY" "$(read_dotenv "$GRAPERY_ENV" "GRAPERY_INTERNAL_API_KEY")"

  # local default: point agent at host grapery, not docker DNS "server"
  current_base="$(read_dotenv "$ENV_FILE" "GRAPERY_BASE_URL")"
  if [ -z "$current_base" ] || [ "$current_base" = "http://server:8080" ]; then
    upsert_env "GRAPERY_BASE_URL" "http://localhost:8080"
  fi
else
  echo "⚠️  grapery .env not found at $GRAPERY_ENV (skip shared sync)"
  echo "   Fill HUOSHAN_API_KEY / AGENT_TOKEN_VERIFY_KEY in $ENV_FILE manually"
fi

# Status (no secret values)
status_key() {
  local key="$1" required="$2"
  local val
  val="$(read_dotenv "$ENV_FILE" "$key")"
  if [ -n "$val" ]; then
    echo "   ✅ $key=set"
  elif [ "$required" = "1" ]; then
    echo "   ❌ $key=MISSING (required for AI / token)"
  else
    echo "   ⚪ $key=empty (optional)"
  fi
}

echo "📋 agent env check ($ENV_FILE):"
status_key "HUOSHAN_API_KEY" 1
status_key "AGENT_TOKEN_VERIFY_KEY" 0
status_key "AGENT_TOKEN_SIGNING_KEY" 0
status_key "GRAPERY_API_KEY" 0
status_key "GRAPERY_BASE_URL" 1
status_key "JWT_SECRET" 0
status_key "EINO_TEXT_MODEL" 0

echo "✅ synced"
