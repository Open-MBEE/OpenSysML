#!/usr/bin/env bash
# Fetch one pinned file over HTTPS only, riding out a rate limit (429) or an outage:
# usage: scripts/fetch-https.sh URL DEST. The caller verifies the digest of DEST.
set -euo pipefail

if [[ $# -ne 2 ]]; then
	echo "usage: $0 URL DEST" >&2
	exit 2
fi

# Eight retries back off 1, 2, 4 ... 128 seconds, or as long as the server's Retry-After
# asks; ten minutes bound the whole, so a host that stays down still fails the caller.
exec curl -fsSL --proto '=https' --proto-redir '=https' --connect-timeout 30 \
	--retry 8 --retry-all-errors --retry-max-time 600 -o "$2" "$1"
