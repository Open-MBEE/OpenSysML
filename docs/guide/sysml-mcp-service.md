# Running sysml-mcp as a service

[`sysml-mcp`](../reference/sysml-mcp.md) is usually started by an agent host over stdin and
stdout, one process per client. With `-http` it is a service instead: started once, serving every
client that connects, and run the way other services are — behind TLS, with a credential in
front, probed, configured from the environment, in a container or under systemd. This page is
the operator's view; the [reference](../reference/sysml-mcp.md#transports) has the transport's
rules, flag by flag, and the [security policy](security.md) the hardening advice that applies to
every served binary.

```bash
sysml-mcp -http 0.0.0.0:8765 -token-file /etc/sysml-mcp/mcp.token -root /srv/models \
    -tls-cert server.crt -tls-key server.key
```

## Deployment topology

The server speaks the Streamable HTTP transport at `/mcp` of the address `-http` names and
answers two open probes, `/health` and `/ready`, beside it. A port alone, or an address with no
host, binds loopback; `0.0.0.0:8765` or a host name serves other machines, and is refused with
exit status 2 unless a credential is configured — a bearer token, a file of API keys, an OpenID
provider, or several of them.

Two shapes work:

- **TLS at the server.** `-tls-cert FILE -tls-key FILE` serve `https` directly, as the command
  above does. Cleartext on an address other machines reach is served, but logged as a warning:
  a bearer token crossing the network in clear can be read and replayed.
- **TLS at a reverse proxy.** The server binds loopback (`-http 8765`) or a private network, and
  Caddy, nginx or an ingress terminates TLS in front of it. The proxy must forward `/mcp`, the
  resource metadata at `/.well-known/oauth-protected-resource/mcp`, and `/health` and `/ready`
  if the probes come through it, and it must not buffer responses: a client's event stream is a
  long-lived `GET /mcp` that stays open as long as its session does (the shipped `Caddyfile`
  sets `flush_interval -1` for that reason).

Behind a proxy the server no longer sees the URL the client used, so **`-public-url`** names it:

```bash
sysml-mcp -http 8765 -token-file mcp.token -public-url https://models.example.org/mcp
```

That URL becomes the `resource` of the RFC 9728 metadata, the `resource_metadata` of every
`401` challenge, the default audience an OpenID token must name, and the URL a
[`-print-config` block](../reference/sysml-mcp.md#a-served-instance-over-http) tells the host to
connect to. With a path prefix the proxy strips (`-public-url https://models.example.org/agents/mcp`),
the metadata is served at both `/.well-known/oauth-protected-resource/agents/mcp` and
`/.well-known/oauth-protected-resource/mcp`, so either forwarding scheme reaches it. A
browser-based host needs its origin in `-allow-origin` as well; the allow-list is not
authentication, it only keeps a page on another site from using a browser as the way in.

## Health and readiness

The server answers two open endpoints on the `-http` address, outside the credential and the
origin check that guard `/mcp`, in the same stance [`sysml-grpc` takes on its main
port](../reference/service-transports.md#health): a probe carries no token, and what it learns is
the build and no more.

| Endpoint | Answer | Meaning |
|---|---|---|
| `GET /health` | `200 {"service":"sysml-mcp","status":"ok","version":"…"}` | **Liveness.** The process is up and its listener bound; answers from the first moment the address is bound. |
| `GET /ready` | `503 {"service":"sysml-mcp","status":"starting"}` | **Not yet ready.** The workspace is being indexed or the runtime's standard library prepared; a `Retry-After: 1` header says to come back. |
| | `200 {"service":"sysml-mcp","status":"ready"}` | **Ready.** A client's first call is answered in full. |
| | `503 {"service":"sysml-mcp","status":"draining"}` | **Stopping.** The server has been told to stop and is closing sessions; a balancer should route elsewhere. |

A protocol request that arrives at `/mcp` before the server is ready is answered 503 with
`Retry-After: 1` and a plain-text reason (`the server is still indexing the workspace and
preparing the runtime; GET /ready answers 200 once it is ready`) rather than served partially.
Point a Kubernetes `livenessProbe` at `/health` and its `readinessProbe` at `/ready`; a Compose
or systemd deployment polls `/ready` once after starting, as the examples below do.

```console
$ curl -s http://localhost:8765/health
{"service":"sysml-mcp","status":"ok","version":"v0.12.1-1072-gebf16f5df5"}
$ curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8765/ready
200
```

Stopping — SIGINT or SIGTERM — turns `/ready` to 503, closes the clients' sessions so their event
streams end, and waits up to `-shutdown-timeout` (default `30s`) for the connections to drain
before closing what is left; the exit status is 0. A request body larger than `-max-body-bytes`
(default `4194304`, 4 MiB) is refused with 413 before it is read in full.

## Configuring from the environment

Every flag is read from the environment as `OPENSYSML_MCP_<FLAG>`: the flag's name in upper case
with each dash an underscore, so `-log-level` is `OPENSYSML_MCP_LOG_LEVEL`, `-http` is
`OPENSYSML_MCP_HTTP` and `-oidc-issuer` is `OPENSYSML_MCP_OIDC_ISSUER`. The rules:

- A flag on the command line wins over its variable.
- A variable set to nothing is as good as unset, so a unit file may leave `OPENSYSML_MCP_TLS_CERT=`
  in place.
- A value the flag would refuse is refused with the flag's own message, naming the variable and
  the flag, and exit status 2 before anything is served:
  `invalid value "many" for OPENSYSML_MCP_CACHE_SIZE (the environment of -cache-size): parse error`.
- `OPENSYSML_MCP_TOKEN` holds the token itself, as it always has, and `OPENSYSML_MCP_TOKEN_FILE`
  names the file holding it; each is read only when neither `-token` nor `-token-file` is given,
  and the file wins over the token, as `-token-file` wins over `-token`.
- `-help`, `-man` and `-version` are answered before the environment is read, so a unit's
  variables never keep `sysml-mcp -version` from printing.

```bash
OPENSYSML_MCP_HTTP=0.0.0.0:8765 OPENSYSML_MCP_TOKEN_FILE=/run/secrets/mcp.token \
OPENSYSML_MCP_PUBLIC_URL=https://models.example.org/mcp sysml-mcp
```

The run bounds every OpenSysML binary reads (`OPENSYSML_MAX_STEPS`, `OPENSYSML_LIBRARY_PATH`, …)
are listed in the [environment reference](../reference/environment.md).

## The container image

`sysml-mcp` ships in the [`openmbee/opensysml`](https://hub.docker.com/r/openmbee/opensysml)
image beside `sysml-grpc` (the entrypoint), `sysml` and `sysml-lsp`; `--entrypoint sysml-mcp`
selects it. The image is distroless and runs as `nonroot` (uid 65532); the one path it writes
is the standard library's record cache under `/home/nonroot/.cache`, so a read-only root
filesystem needs a volume there that `nonroot` can write — a fresh named volume is owned by
root, so the first start says once that the record cache is unavailable and holds every
document loaded instead, which is slower to start but otherwise the same server.

```bash
docker run --rm -p 8765:8765 -v "$PWD:/models:ro" -e OPENSYSML_MCP_TOKEN="$(cat mcp.token)" \
    --read-only --tmpfs /tmp -v opensysml-cache:/home/nonroot/.cache \
    --entrypoint sysml-mcp openmbee/opensysml -http 0.0.0.0:8765 -root /models
```

`deploy/sysml-mcp/compose.yaml` in the repository is the complete example: `sysml-mcp` on a
private network behind [Caddy](https://caddyserver.com/), which terminates TLS and forwards
`/mcp`, `/health` and `/ready`; the token as a Docker secret read from `secrets/mcp.token` through
`OPENSYSML_MCP_TOKEN_FILE`; the models mounted read-only from `./models`; the root filesystem
read-only with every capability dropped; `OPENSYSML_MCP_PUBLIC_URL` naming the proxied URL, host
and published port, so the bearer challenge and the resource metadata name what clients see.
`MCP_HOST` (default `localhost`), `MCP_HTTPS_PORT` (default `8443`) and `OPENSYSML_IMAGE` are
the knobs.

```bash
cd deploy/sysml-mcp
openssl rand -base64 32 > secrets/mcp.token
docker compose up -d
curl -sk https://localhost:8443/health
curl -sk https://localhost:8443/ready
```

To serve API keys or single sign-on from the container, add the matching variables to the
service's `environment` — `OPENSYSML_MCP_API_KEYS=/run/secrets/keys.jsonl` with the file as a
second secret (mint keys with the same binary on the host, then restart or `kill -HUP` the
container's process), or `OPENSYSML_MCP_OIDC_ISSUER` — and drop `OPENSYSML_MCP_TOKEN_FILE` when
the static token is no longer wanted.

## The systemd unit

`deploy/sysml-mcp/systemd/sysml-mcp.service` runs the binary under its own user with the
hardening systemd offers (`ProtectSystem=strict`, no new privileges, an empty capability set, a
private `/tmp`), reading every setting from `/etc/sysml-mcp/sysml-mcp.env` — the example
`sysml-mcp.env` beside it names the address, the token file, the public URL, the workspace, the
log level and the bounds, and leaves the TLS pair and the origins empty, which is unset. The
unit's `TimeoutStopSec=45` allows for `OPENSYSML_MCP_SHUTDOWN_TIMEOUT=30s` before SIGKILL; raise
both together. The record cache lands in the unit's own `/var/cache/sysml-mcp`.
`deploy/sysml-mcp/README.md` lists the installation steps: a system user, the token file readable
by that user only, the unit and the environment file copied into place, `systemctl enable --now`.

```bash
systemctl status sysml-mcp
journalctl -u sysml-mcp -f
curl -s http://127.0.0.1:8765/ready
```

The audit log below goes to stderr, so `journalctl -u sysml-mcp` is where it lands; the unit's
`OPENSYSML_MCP_LOG_LEVEL=info` is what makes the server write it.

## Authentication

The server is an OAuth 2.1 resource server: every request to `/mcp` carries
`Authorization: Bearer …` (RFC 6750 § 2.1, the only form read), and three authenticators share
that one header. A request is tried against them in a fixed order, and the first that recognizes
the credential answers for it:

1. **API keys** (`-api-keys FILE`) read a token with the `osk_` prefix, and no other.
2. **Single sign-on** (`-oidc-issuer URL`) reads a token that is a JWT — or, with
   `-oidc-introspect`, any token that is not an API key.
3. **The static token** (`-token-file`, `-token`, `OPENSYSML_MCP_TOKEN`) reads whatever is left.

Any combination is allowed, and an address other machines reach needs at least one. Choose in
this order: single sign-on when people use the server and an identity provider already knows
them; API keys for CI jobs, other agents and people without a provider, one key per caller so
that one can be revoked alone; the static token for loopback, a development machine, or a
single trusted proxy that holds it. Whichever authenticates, the caller becomes a **principal**
— `source:subject`, as `oidc:28b78b21-…`, `api-key:alice@example.org` or `static-token:token` —
and that identity is what scopes, sessions and the audit log are about.

### Single sign-on (OIDC)

```bash
sysml-mcp -root /srv/models -http 127.0.0.1:8765 -public-url https://models.example.org/mcp \
    -oidc-issuer https://login.example.org/realms/opensysml
```

- **What is verified.** A bearer token that is a JWT is checked against the provider: a
  signature under one of its keys (RS, ES, PS or EdDSA, from the `jwks_uri` of its discovery
  document), `iss` equal to `-oidc-issuer`, `aud` holding this server's audience, `exp` in the
  future and `nbf` in the past (30 seconds of clock skew allowed either way), and a `sub`. The
  audience is the URL of `-public-url` unless `-oidc-audience AUD` names another; `-oidc-issuer`
  with neither is refused with exit status 2
  (`-oidc-issuer needs the audience a token is issued for: name the endpoint with -public-url, or give -oidc-audience`).
- **Who the caller is.** The principal is `oidc:<sub>`, named by `preferred_username`, else
  `email`, else `name`; its scopes come from the `scope` claim (space-separated) or `scp`
  (array), or from the claim `-oidc-scope-claim NAME` names.
- **Discovery is lazy.** The provider's `/.well-known/openid-configuration` is read when the
  first JWT arrives and kept; until it can be read, and whenever the provider cannot be reached,
  such a token is answered `503 Service Unavailable` with a challenge carrying no error code, so
  the host retries rather than signing in again. A provider that is down never stops the server
  from starting.
- **How a host finds the provider.** With `-oidc-issuer` the resource metadata at
  `/.well-known/oauth-protected-resource/mcp` lists it in `authorization_servers` and the
  server's scopes in `scopes_supported`; a host that follows the `401` challenge's
  `resource_metadata` runs Authorization Code + PKCE against the provider and comes back with a
  token. The server never sees a password.
- **Opaque tokens.** A provider whose access tokens are not JWTs is asked about them through
  RFC 7662 introspection: `-oidc-introspect URL -oidc-client-file FILE`, the file a JSON object
  with the `client_id` and `client_secret` of a confidential client allowed to introspect. An
  active answer is kept until the token's `exp`.

```console
$ curl -s https://models.example.org/.well-known/oauth-protected-resource/mcp
{"resource":"https://models.example.org/mcp","authorization_servers":["https://login.example.org/realms/opensysml"],"scopes_supported":["sysml:read","sysml:write","sysml:execute","sysml:session","sysml:admin"],"bearer_methods_supported":["header"],"resource_name":"sysml-mcp","resource_documentation":"https://github.com/Open-MBEE/OpenSysML/blob/main/docs/reference/sysml-mcp.md"}
```

**Keycloak walkthrough.** The repository ships a development realm,
`deploy/keycloak/opensysml-realm.json`: realm `opensysml`, client scopes `sysml:read` and
`sysml:write`, a public client `mcp-inspector` with PKCE and the Inspector's redirect URIs, an
audience mapper that puts `http://localhost:8765/mcp` into the access token, and a user
`alice` with password `alice`.

```bash
docker run --rm -p 8080:8080 \
  -e KC_BOOTSTRAP_ADMIN_USERNAME=admin -e KC_BOOTSTRAP_ADMIN_PASSWORD=admin \
  -v "$PWD/deploy/keycloak:/opt/keycloak/data/import:ro" \
  quay.io/keycloak/keycloak:26.4 start-dev --import-realm

sysml-mcp -root examples -http 127.0.0.1:8765 -public-url http://localhost:8765/mcp \
  -oidc-issuer http://localhost:8080/realms/opensysml

npx @modelcontextprotocol/inspector --server-url http://localhost:8765/mcp --transport http
```

Connect in the Inspector: the `401` leads it to the metadata, the metadata to Keycloak, and
Keycloak's login page (alice / alice) back to the Inspector with a token the server accepts;
`workspace_info` then answers, and a tool outside the token's scopes is refused as
[below](#scopes-and-per-principal-sessions). An `http` issuer is accepted on loopback only, for
a provider run beside the server; a real deployment names an `https` issuer. For your own realm
the client needs the host's redirect URIs, an *Audience* mapper putting the server's
`-public-url` into the access token, and client scopes named after the server's scopes with
*Include in token scope* on.

**Other providers** differ in the audience they put in `aud` and the claim they put scopes in:

- *Microsoft Entra ID.* Register the server as an application that exposes an API; its
  *Application ID URI* (`api://<client-id>`) is the `aud`, so pass it as `-oidc-audience`.
  Scopes arrive in `scp` (space-separated) and app roles in `roles`; `scp` is read by default.
  A token for the Microsoft Graph audience cannot be verified by anyone else, so the host must
  request the API's scopes, not Graph's.
- *Okta.* Use a custom authorization server (the org server's tokens are not verifiable by
  third parties); its *Audience* setting is the `aud` value — set it to the server's resource URL
  or pass it as `-oidc-audience`. Scopes come in `scp` as an array, read by default; the issuer
  is `https://<org>.okta.com/oauth2/<server-id>`.
- *Ping (PingFederate / PingOne).* Add the resource URL to the access token's audience through
  the access token manager (PingFederate) or the resource's audience (PingOne); scopes are in
  `scope`, space-separated. A deployment that maps them into another claim names it with
  `-oidc-scope-claim`.

### API keys

An API key is a per-caller bearer token the operator mints and revokes; the server's key store
is a file, and the same binary edits it.

```bash
sysml-mcp -mint-api-key -api-keys keys.jsonl -subject alice@example.org \
    -scopes sysml:read,sysml:write -expires 720h
sysml-mcp -mint-api-key -api-keys keys.jsonl -subject ci@example.org -scopes sysml:read
sysml-mcp -revoke-api-key Z7oataRyhNcR -api-keys keys.jsonl
sysml-mcp -http 0.0.0.0:8765 -api-keys keys.jsonl -tls-cert server.crt -tls-key server.key
```

- **Minting** prints the key once, to stdout alone, and appends its record to the file (created
  with mode `0600` when absent). The key is `osk_<id>.<secret>`: the `osk_` prefix tells it from
  any other bearer token, the id (12 characters) names its record, and the secret is 32 random
  bytes in base64url. The file holds the SHA-256 of the secret, never the secret, so a copy of
  the file gives no key away and a lost key is minted again rather than recovered. `-expires`
  is a Go duration (`720h`, `90m`); without it the key does not expire.

    ```console
    $ sysml-mcp -mint-api-key -api-keys keys.jsonl -subject ci@example.org -scopes sysml:read
    sysml-mcp: key Z7oataRyhNcR for ci@example.org (does not expire) appended to keys.jsonl; it is printed once, the file holds only its hash
    osk_Z7oataRyhNcR.JZ6jEZ2P9iyCr49nJH75GgVbGOSrEXImZKFRLFbDkGo
    ```

- **The file** is JSON Lines, one record per line:
  `{"id","sha256","subject","scopes","expires","disabled"}` — the hash in hex, the subject the
  key speaks for, the scopes it grants (at least one, RFC 6749 scope tokens), its expiry as an
  RFC 3339 time (absent for a key that does not expire) and `"disabled":true` once revoked. A
  line that is not a record, an unknown scope syntax or an id held twice fails the file at that
  line, and the server keeps the keys it already held.

    ```json
    {"id":"OdC1anYU83CX","sha256":"c11aeb1e…b207d7","subject":"alice@example.org","scopes":["sysml:read","sysml:write"],"expires":"2026-12-10T11:53:51Z"}
    {"id":"Z7oataRyhNcR","sha256":"f4b96712…cefee8","subject":"ci@example.org","scopes":["sysml:read"],"disabled":true}
    ```

- **Revocation and rotation.** `-revoke-api-key ID` marks the record disabled; the line stays,
  so the subject and the history are kept, and the key is refused from the server's next look at
  the file as `401` `invalid_token`. To rotate, mint the subject a new key, move the caller to
  it, then revoke the old id. A key past its `expires` is refused as expired without being
  revoked. Two edits at once wait for each other through `keys.jsonl.lock` beside the file.
- **Hot reload.** A serving instance re-reads the file when its modification time moves — at
  most once a second, and only when a request arrives — and on `SIGHUP`, so `kill -HUP` the
  server after minting or revoking to have it read at once. A file that no longer parses keeps
  the keys already held and logs the failure once.
- **The principal** is `api-key:<subject>` with the record's scopes and expiry. The key is
  compared by constant-time hash, is never logged, and the refusal never repeats it.

Give a host the key as its `Authorization: Bearer osk_…` header; the
[per-host blocks](../reference/sysml-mcp.md#a-served-instance-over-http) show where each puts it.

### Static bearer token

One token, issued by whoever runs the server, that every request may carry; `-token-file FILE`
(preferred: the trailing line break is dropped and the token stays out of the process list),
`-token TOKEN`, or `OPENSYSML_MCP_TOKEN` / `OPENSYSML_MCP_TOKEN_FILE` in the environment. The
token is an RFC 6750 `b64token` — letters, digits and `-._~+/` with trailing `=` — so
`openssl rand -base64 32` makes one.

```bash
openssl rand -base64 32 > mcp.token
sysml-mcp -http 8765 -token-file mcp.token          # loopback, token enforced
sysml-mcp -http 0.0.0.0:8765 -token-file mcp.token -tls-cert server.crt -tls-key server.key
```

Every holder of the token is the same principal, `static-token:token`, and that principal holds
**every scope, `sysml:admin` included**: the token is the operator's own credential, right for
loopback, a development machine or one trusted proxy, and wrong for a server several people
share, where the audit log would show one name and nobody could be revoked alone. Without any
credential a loopback server authenticates nobody and every client is the one `local` principal,
as the stdio transport's host is. `-print-config HOST -http ADDR -token-file FILE` writes the
host's block with the `Authorization` header filled in, in clear, for a file the host reads
privately.

### Scopes and per-principal sessions

Every tool of the catalog needs exactly one scope, and a principal's scopes come from its
credential: the API key's record, the OpenID token's `scope`/`scp` claim, or all of them for the
static token and the local principal. `tools/list` is not filtered — every tool is listed — and
a `tools/call` outside the principal's scopes is refused with the JSON-RPC error
`-32030` naming the tool and the scope, never served in part.

| Scope | Admits | For example |
|---|---|---|
| `sysml:read` | Asking about the workspace and the model: queries, diagnostics, hover, rendering, conversions, verification, and edits answered as text that change nothing on disk | `workspace_diagnostics`, `hover`, `render_view`, `symbol`, `evaluate`, `convert`, `apply_edits`, `verify_requirement`, `session_print`, `tool_catalog` |
| `sysml:write` | Writing the workspace: files replaced with `apply`, documents held open or released, a session's model saved | `format`, `rename`, `apply_model_edit`, `import_diagram`, `open_document`, `update_document`, `close_document`, `session_save` |
| `sysml:execute` | Executing the model: actions, state machines, analyses, sweeps, solving, debugging and REPL evaluation | `execute_action`, `run_analysis`, `instantiate`, `check_behavior`, `debug_start`, `send`, `solve`, `session_run`, `session_eval` |
| `sysml:session` | The caller's own REPL sessions: open, load, submit, list, clear and close | `session_open`, `session_load`, `session_submit`, `session_list`, `session_clear`, `session_close` |
| `sysml:admin` | Other principals' sessions: `session_list` answers every principal's with its owner, `session_close` closes another's by `owner`; no tool needs it on its own | — |

The [reference](../reference/sysml-mcp.md#scopes) lists every tool under its scope.
`repl_command` is admitted with `sysml:read` and then takes the scope of the typed tool its `%`
command corresponds to, so `%action` through it needs `sysml:execute` as `session_run` does. An
API key is minted with exactly the scopes its caller needs; an OpenID client requests them, and
the provider grants what it is configured to.

```console
$ # tools/call session_open with a key holding sysml:read,sysml:write
{"jsonrpc":"2.0","id":2,"error":{"code":-32030,"message":"insufficient_scope: session_open requires the scope sysml:session, which this credential does not grant","data":{"tool":"session_open","scope":"sysml:session"}}}
```

REPL sessions belong to the principal that opened them. A `session_*` name is private to its
owner: two agents holding the same credential share the session they both call `work`, and
another principal's `work` is a different session, invisible to the first. Two protocol
sessions are kept apart the same way — an `Mcp-Session-Id` is honoured only for the principal
whose `initialize` created it, `403 Forbidden` to any other, so a credential cannot be swapped
under a running session.

The administrator's path is the `sysml:admin` scope: `session_list` then answers `all`, every
principal's open sessions each with its `owner` (`source:subject`), and `session_close` takes
an `owner` beside the `session` name and closes that principal's session. A caller without the
scope gets its own sessions from `session_list` and is refused the `owner` argument
(`closing another principal's session takes the scope sysml:admin, which this credential does not grant`).

```console
$ # session_list as ops@example.org, whose key holds sysml:session,sysml:admin
{"sessions":["work"],"all":[{"owner":"api-key:ops@example.org","session":"work"},{"owner":"static-token:token","session":"work"}]}
$ # session_close {"session":"work","owner":"static-token:token"}
{"session":"work","owner":"static-token:token"}
```

### Audit log

Every `tools/call` writes one line to stderr at level `INFO` (so `-log-level info`, or
`OPENSYSML_MCP_LOG_LEVEL=info`, is what turns the audit on; the default `warn` keeps it quiet),
in the `key=value` form of Go's `slog` text handler:

```text
time=2026-11-10T11:53:51.435Z level=INFO msg=tools/call principal=ci@example.org source=api-key remote=127.0.0.1:32958 tool=session_open scope=sysml:session outcome=refused duration=71.763µs
time=2026-11-10T11:53:51.454Z level=INFO msg=tools/call principal=ci@example.org source=api-key remote=127.0.0.1:32966 tool=workspace_info scope=sysml:read outcome=ok duration=2.596903ms
```

| Field | Meaning |
|---|---|
| `principal` | The subject of the credential: an OpenID `sub`, an API key's subject, `token` for the static token, `local` when nobody is authenticated |
| `source` | Which authenticator admitted it: `oidc`, `api-key`, `static-token` or `local` |
| `remote` | The peer's address as the server saw it (the proxy's, behind one) |
| `tool` | The tool called |
| `scope` | The scope the tool needs |
| `outcome` | `ok`; `refused` when the principal lacks the scope (the `-32030` above); `error` when the tool itself answered an error |
| `duration` | How long the call took |

The credential itself and the call's arguments are never written, so the log can be kept and
shipped without becoming a secret. A request refused at the HTTP layer (no token, a wrong one)
never reaches `tools/call` and leaves no line here; the client's `401` is the record of it.

## Troubleshooting

Every refusal of `/mcp` is an RFC 6750 § 3 challenge: the status, the `WWW-Authenticate` header
and a one-line body say what was wrong, and the `resource_metadata` of the header names the
document a host reads to find out how to authenticate.

| Answer | Challenge / body | Meaning |
|---|---|---|
| `401 Unauthorized` | `Bearer realm="sysml-mcp", resource_metadata="…"` — `Unauthorized: this server requires a bearer token; send Authorization: Bearer TOKEN` | No `Authorization` header. The host has not authenticated yet; one that follows the metadata starts sign-in (OIDC) or learns the token is preconfigured. |
| `401` | `error="invalid_token", error_description="the bearer token is not this server's"` | A well-formed token nobody accepts: the wrong static token, an unknown or **revoked** API key, a JWT for another issuer or audience or with a bad signature, or an opaque token with no introspection configured. |
| `401` | `error="invalid_token", error_description="the bearer token has expired"` | An API key past `expires`, or a JWT past `exp`. The host should obtain a fresh token; the key needs minting again. |
| `401` | `error="invalid_request", error_description="the Authorization scheme must be Bearer"` | `Authorization: Basic …` or another scheme. |
| `400 Bad Request` | `error="invalid_request", error_description="the bearer token is malformed"` | A `Bearer` header whose credentials are not a `b64token` (spaces, quotes, line breaks). |
| `403 Forbidden` | `Forbidden: origin "…" is not allowed; this server answers loopback origins and those named by -allow-origin` | A browser origin outside the allow-list. Add it to `-allow-origin`; this is not a credential problem. |
| `403` | `session user mismatch` | An `Mcp-Session-Id` of a protocol session another principal opened. Each client initializes its own session. |
| `503 Service Unavailable` | `Bearer realm="sysml-mcp", resource_metadata="…"` (no `error`) — `Service Unavailable: the bearer token cannot be verified for now` | An OpenID token the server cannot verify because the provider's discovery document or keys are out of reach. Not the client's fault: retry, and check the server's path to `-oidc-issuer`. |
| `503` + `Retry-After: 1` | `the server is still indexing the workspace and preparing the runtime; GET /ready answers 200 once it is ready` | Started but not ready, or draining. Wait for `/ready`. |
| `413` | | The request body exceeds `-max-body-bytes`. |
| JSON-RPC `-32030` | `insufficient_scope: <tool> requires the scope <scope>, which this credential does not grant` | Authenticated, but the credential lacks the tool's scope. Mint a key with it, or have the provider grant it. |

And at the command line:

- **Exit status 2 before serving** is a refused configuration, with the reason on stderr: an
  address other machines reach with no credential (`-http 0.0.0.0:8765 binds an address other
  machines reach: no bearer token is configured; name one with -token-file, -token or
  OPENSYSML_MCP_TOKEN, or bind loopback` — an `-api-keys` file or `-oidc-issuer` satisfies it as
  well), `-oidc-issuer` without `-public-url` or `-oidc-audience`, one of `-tls-cert`/`-tls-key`
  without the other, a variable the flag would refuse.
- **Exit status 1** is a server that could not start (the `-http` address in use) or a session
  that ended in a protocol error; **0** is a server stopped by SIGINT or SIGTERM.
- **A host signs in but every tool is refused** with `-32030`: the provider did not put the
  server's scopes in the token. Check the client scopes (Keycloak: *Include in token scope*),
  the claim they land in (`-oidc-scope-claim`), and that the host requested them.
- **A host loops on `401` after signing in**: the token's `aud` is not the server's. Compare the
  provider's audience mapper with `-public-url`, or name the audience with `-oidc-audience`.
- **The audit log is empty**: the server runs at the default `-log-level warn`; set `info`.
