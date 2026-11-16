#!/usr/bin/env bash
# Fetch one pinned file over HTTPS only, riding out a rate limit (429) or an outage:
# usage: scripts/fetch-https.sh URL DEST. The caller verifies the digest of DEST.
set -euo pipefail

if [[ $# -ne 2 ]]; then
	echo "usage: $0 URL DEST" >&2
	exit 2
fi

# Eight retries back off 1, 2, 4 ... 128 seconds, or as the server's Retry-After asks, and none
# starts after ten minutes. An attempt ends at five minutes, or once it stalls for 30 seconds.
exec curl -fsSL --proto '=https' --proto-redir '=https' --connect-timeout 30 \
  --max-time 300 --speed-limit 1024 --speed-time 30 \
  --retry 8 --retry-all-errors --retry-max-time 600 -o "$2" "$1"
