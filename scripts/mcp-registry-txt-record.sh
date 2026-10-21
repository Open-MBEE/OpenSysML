#!/usr/bin/env bash
# Print the DNS TXT record and the mcp-publisher login that prove opensysml.org
# to the MCP Registry, from an Ed25519 private key the maintainer already holds.
#
#   openssl genpkey -algorithm Ed25519 -out mcp-registry-key.pem   # once, offline
#   scripts/mcp-registry-txt-record.sh mcp-registry-key.pem
#
# The key is only read. Nothing here generates, copies or stores it, and the
# private half is never written to the terminal: the login line below derives
# it from the file at the moment it runs. The derivations are the ones in
# https://modelcontextprotocol.io/registry/authentication (DNS Authentication).
set -euo pipefail

DOMAIN="${MCP_REGISTRY_DOMAIN:-opensysml.org}"

usage() {
  echo "usage: $0 PRIVATE_KEY.pem" >&2
  echo "       MCP_REGISTRY_DOMAIN overrides the domain (default opensysml.org)" >&2
  exit 2
}

[[ $# -eq 1 ]] || usage
key="$1"
if [[ ! -r "$key" ]]; then
  echo "Error: cannot read private key file $key" >&2
  exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
  echo "Error: openssl is not on PATH; the record is derived with it." >&2
  exit 1
fi

algorithm="$(openssl pkey -in "$key" -noout -text 2>/dev/null | head -n 1 || true)"
if [[ "$algorithm" != "ED25519 Private-Key:" ]]; then
  echo "Error: $key is not an Ed25519 private key (openssl pkey reports '${algorithm:-nothing}')." >&2
  echo "       Generate one with: openssl genpkey -algorithm Ed25519 -out $key" >&2
  exit 1
fi

# The DER SubjectPublicKeyInfo is a fixed 12-byte prefix (16 base64 characters) followed by the
# raw 32-byte Ed25519 public key, so its base64 from character 17 on is the key's base64.
public_key="$(openssl pkey -in "$key" -pubout -outform DER | openssl base64 -A | cut -c17-)"

cat <<RECORD
# DNS TXT record for $DOMAIN (one record at the zone apex, next to any existing TXT records):
${DOMAIN}. IN TXT "v=MCPv1; k=ed25519; p=${public_key}"

# Check propagation (the p= value must appear, from every resolver the registry may ask):
dig TXT ${DOMAIN} +short
dig @1.1.1.1 TXT ${DOMAIN} +short

# Log in once the record resolves. The private key is the 64-hex-digit seed, read from the file
# as the command runs; mcp-publisher stores the resulting token under ~/.config/mcp-publisher/.
mcp-publisher login dns --domain "${DOMAIN}" --private-key "\$(openssl pkey -in "${key}" -noout -text | awk '/^priv:/ { seed = 1; next } /^pub:/ { seed = 0 } seed' | tr -d ' :\\n')"

# GitHub Actions secret MCP_REGISTRY_PRIVATE_KEY: the same 64 hex digits, produced by
openssl pkey -in "${key}" -noout -text | awk '/^priv:/ { seed = 1; next } /^pub:/ { seed = 0 } seed' | tr -d ' :\\n'
RECORD
