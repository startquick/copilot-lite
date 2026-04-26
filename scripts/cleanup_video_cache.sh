#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="${CACHE_DIR:-$ROOT_DIR/data/tmp/video}"
AGE_HOURS="${AGE_HOURS:-8}"

# Keep behavior predictable: if AGE_HOURS is invalid, fallback to 8.
if ! [[ "$AGE_HOURS" =~ ^[0-9]+$ ]]; then
  AGE_HOURS=8
fi

mkdir -p "$CACHE_DIR"

# Delete only regular files older than AGE_HOURS in the video cache directory.
DELETED_COUNT=$(find "$CACHE_DIR" -type f -mmin +$((AGE_HOURS * 60)) -print -delete | wc -l | tr -d ' ')

printf '%s cleanup_video_cache: dir=%s age_hours=%s deleted=%s\n' \
  "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$CACHE_DIR" "$AGE_HOURS" "$DELETED_COUNT"
