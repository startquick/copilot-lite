#!/bin/sh
set -eu

CONFIG_PATH="${GROKPI_CONFIG_PATH:-/app/config.toml}"
DEFAULT_APP_KEY="QUICKstart012345+"

if [ ! -f "$CONFIG_PATH" ]; then
  echo "ERROR: missing config file at $CONFIG_PATH" >&2
  echo "Mount a production config.toml before starting the container." >&2
  exit 1
fi

if grep -Eq '^[[:space:]]*app_key[[:space:]]*=[[:space:]]*"'$DEFAULT_APP_KEY'"[[:space:]]*$' "$CONFIG_PATH"; then
  echo "ERROR: config file still uses the default admin app_key." >&2
  echo "Set a unique app.app_key in $CONFIG_PATH before starting the container." >&2
  exit 1
fi

exec /usr/local/bin/grokpi "$@"
