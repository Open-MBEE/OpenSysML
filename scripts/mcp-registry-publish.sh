#!/usr/bin/env bash
#
# List a release of sysml-mcp in the MCP Registry as org.opensysml/sysml-mcp.
#
# Usage: scripts/mcp-registry-publish.sh <tag>            publish the tag's version
#        scripts/mcp-registry-publish.sh --check [<tag>]  validate only, no credentials
#
# The registry holds metadata only: packaging/mcp/server.json points at the npm
# package @openmbee/sysml-mcp, which CircleCI publishes on the same tag. So the
# script waits for that version to appear on registry.npmjs.org, installs the
# pinned mcp-publisher, fills the tag's version into a copy of server.json,
# proves opensysml.org with the Ed25519 key in MCP_REGISTRY_PRIVATE_KEY (PEM as
# `openssl genpkey -algorithm Ed25519` writes it, or the 64-hex-digit seed the
# CLI takes) and publishes. --check stops after mcp-publisher's own validation
# of the filled file, which needs no login; `make mcp-registry-check` runs it.
#
# Environment:
#   MCP_REGISTRY_PRIVATE_KEY   the key (publishing only)
#   MCP_REGISTRY_URL           registry to publish to (default the production registry;
#                              https://staging.registry.modelcontextprotocol.io is the
#                              registry's staging instance, whose data may be reset)
#   MCP_REGISTRY_NPM_TIMEOUT   seconds to wait for the npm version (default 3600)
#   MCP_REGISTRY_ALLOW_PRERELEASE  publish a pre-release tag too (set for staging rehearsals)
set -euo pipefail

SERVER_JSON="packaging/mcp/server.json"
DOMAIN="opensysml.org"
DOCS="docs/project/releasing.md § MCP Registry"
REGISTRY_URL="${MCP_REGISTRY_URL:-https://registry.modelcontextprotocol.io}"
NPM_TIMEOUT="${MCP_REGISTRY_NPM_TIMEOUT:-3600}"
BUILD_DIR="build/mcp-registry"

fail() {
  echo "Error: $*" >&2
  exit 1
}

check=false
if [[ "${1:-}" == "--check" ]]; then
  check=true
  shift
fi
tag="${1:-}"
if [[ -z "$tag" ]]; then
  $check || fail "a release tag is required (usage: $0 [--check] <tag>)"
  tag="v0.0.0-check"
fi
[[ "$tag" =~ ^v([0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?)$ ]] || fail "tag $tag is not vX.Y.Z[-pre]; the registry lists release versions only"
version="${BASH_REMATCH[1]}"
prerelease=false
[[ -n "${BASH_REMATCH[2]}" ]] && prerelease=true

for tool in jq curl openssl; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is not on PATH"
done
[[ -f "$SERVER_JSON" ]] || fail "$SERVER_JSON is missing; run from the repository root"

identifier="$(jq -r '.packages[0].identifier' "$SERVER_JSON")"
name="$(jq -r '.name' "$SERVER_JSON")"

if ! $check; then
  if $prerelease && [[ -z "${MCP_REGISTRY_ALLOW_PRERELEASE:-}" ]]; then
    echo "$tag is a pre-release; the registry lists releases only, so nothing is published for it."
    exit 0
  fi
  [[ -n "${MCP_REGISTRY_PRIVATE_KEY:-}" ]] || fail "MCP_REGISTRY_PRIVATE_KEY is not set; add the repository secret holding the opensysml.org Ed25519 key ($DOCS)"
  key="$(printf '%s' "$MCP_REGISTRY_PRIVATE_KEY" | tr -d '[:space:]')"
  if [[ "$key" == -----BEGIN* ]]; then
    key="$(printf '%s\n' "$MCP_REGISTRY_PRIVATE_KEY" | openssl pkey -noout -text 2>/dev/null | awk '/^priv:/ { seed = 1; next } /^pub:/ { seed = 0 } seed' | tr -d ' :\n')" \
      || fail "MCP_REGISTRY_PRIVATE_KEY is not a PEM private key openssl can read ($DOCS)"
  fi
  [[ "$key" =~ ^[0-9a-fA-F]{64}$ ]] || fail "MCP_REGISTRY_PRIVATE_KEY is neither an Ed25519 PEM private key nor its 64-hex-digit seed, which mcp-publisher login dns takes ($DOCS)"

  # CircleCI's publish-npm job publishes the package on the same tag; the registry rejects
  # a listing whose package is not on npm yet or does not declare the server name.
  encoded="${identifier/\//%2F}"
  packument="$BUILD_DIR/npm-$version.json"
  mkdir -p "$BUILD_DIR"
  deadline=$((SECONDS + NPM_TIMEOUT))
  until curl --proto '=https' --tlsv1.2 -fsSL -o "$packument" "https://registry.npmjs.org/$encoded/$version" 2>/dev/null; do
    if [[ "$SECONDS" -ge "$deadline" ]]; then
      fail "$identifier@$version is not on registry.npmjs.org after ${NPM_TIMEOUT}s; CircleCI's publish-npm job publishes it on tag $tag. Check that job, then re-run this workflow."
    fi
    echo "Waiting for $identifier@$version on registry.npmjs.org..."
    sleep 30
  done
  mcp_name="$(jq -r '.mcpName // ""' "$packument")"
  [[ "$mcp_name" == "$name" ]] || fail "$identifier@$version declares mcpName '${mcp_name:-<none>}' on npm; the registry requires '$name' in its package.json"
fi

publisher="$(scripts/install-mcp-publisher.sh "$BUILD_DIR/publisher")"
filled="$BUILD_DIR/server.json"
mkdir -p "$BUILD_DIR"
jq --arg v "$version" '.version = $v | .packages[].version = $v' "$SERVER_JSON" > "$filled"

echo "Validating $filled against $REGISTRY_URL..."
"$publisher" validate "$filled"
if $check; then
  echo "server.json is valid for $name $version; publishing needs MCP_REGISTRY_PRIVATE_KEY and the npm package."
  exit 0
fi

if ! "$publisher" login dns --domain "$DOMAIN" --private-key "$key" --registry "$REGISTRY_URL"; then
  fail "DNS login for $DOMAIN at $REGISTRY_URL failed. The TXT record 'v=MCPv1; k=ed25519; p=<public key>' for this key must resolve (dig TXT $DOMAIN +short; scripts/mcp-registry-txt-record.sh prints it) and MCP_REGISTRY_PRIVATE_KEY must be the matching key ($DOCS)"
fi
trap '"$publisher" logout >/dev/null 2>&1 || true' EXIT

if ! "$publisher" publish "$filled"; then
  fail "publishing $name $version to $REGISTRY_URL failed; the registry checks that $identifier@$version is on npm with mcpName '$name' and that the version is new"
fi
echo "Published $name $version to $REGISTRY_URL."
