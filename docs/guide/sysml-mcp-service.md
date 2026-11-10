# Running sysml-mcp as a service

[`sysml-mcp`](../reference/sysml-mcp.md) is usually started by an agent host over stdin and
stdout, one process per client. With `-http` it is a service instead: started once, serving every
client that connects, and run the way other services are — probed, configured from the
environment, in a container or under systemd, with a token (and later other credentials) in front.
This page is the operator's view; the [reference](../reference/sysml-mcp.md#transports) has the
transport's rules and the [security policy](security.md) the hardening advice that applies to
every served binary.

```bash
sysml-mcp -http 0.0.0.0:8765 -token-file /etc/sysml-mcp/mcp.token -root /srv/models \
    -tls-cert server.crt -tls-key server.key
```

## Health and readiness

The server answers two open endpoints on the `-http` address, outside the bearer token and the
origin check that guard `/mcp`, in the same stance [`sysml-grpc` takes on its main
port](../reference/service-transports.md#health): a probe carries no token, and what it learns is
the build and no more.

| Endpoint | Answer | Meaning |
|---|---|---|
| `GET /health` | `200 {"service":"sysml-mcp","status":"ok","version":"0.2.1"}` | **Liveness.** The process is up and its listener bound; answers from the first moment the address is bound. |
| `GET /ready` | `503 {"service":"sysml-mcp","status":"starting"}` | **Not yet ready.** The workspace is being indexed or the runtime's standard library prepared; a `Retry-After: 1` header says to come back. |
| | `200 {"service":"sysml-mcp","status":"ready"}` | **Ready.** A client's first call is answered in full. |
| | `503 {"service":"sysml-mcp","status":"draining"}` | **Stopping.** The server has been told to stop and is closing sessions; a balancer should route elsewhere. |

A protocol request that arrives at `/mcp` before the server is ready is answered 503 with
`Retry-After` rather than served partially. Point a Kubernetes `livenessProbe` at `/health` and
its `readinessProbe` at `/ready`; a Compose or systemd deployment polls `/ready` once after
starting, as the examples below do.

```console
$ curl -s http://localhost:8765/health
{"service":"sysml-mcp","status":"ok","version":"0.2.1"}
$ curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8765/ready
200
```

Stopping — SIGINT or SIGTERM — turns `/ready` to 503, closes the clients' sessions so their event
streams end, and waits up to `-shutdown-timeout` (default `30s`) for the connections to drain
before closing what is left. A request body larger than `-max-body-bytes` (default 4 MiB) is
refused with 413 before it is read in full.

## Configuring from the environment

Every flag is read from the environment as `OPENSYSML_MCP_<FLAG>`: the flag's name in upper case
with each dash an underscore, so `-log-level` is `OPENSYSML_MCP_LOG_LEVEL` and `-http` is
`OPENSYSML_MCP_HTTP`. The rules:

- A flag on the command line wins over its variable.
- A variable set to nothing is as good as unset, so a unit file may leave `OPENSYSML_MCP_STRICT=`
  in place.
- A value the flag would refuse — `OPENSYSML_MCP_CACHE_SIZE=many` — is refused with the flag's
  own message and exit status 2, before anything is served.
- `OPENSYSML_MCP_TOKEN` holds the token itself, as it always has, and `OPENSYSML_MCP_TOKEN_FILE`
  names the file holding it; each is read only when neither `-token` nor `-token-file` is given,
  and the file wins over the token, as `-token-file` wins over `-token`.

```bash
OPENSYSML_MCP_HTTP=0.0.0.0:8765 OPENSYSML_MCP_TOKEN_FILE=/run/secrets/mcp.token \
OPENSYSML_MCP_PUBLIC_URL=https://models.example.org/mcp sysml-mcp
```

The run bounds every OpenSysML binary reads (`OPENSYSML_MAX_STEPS`, `OPENSYSML_LIBRARY_PATH`, …)
are listed in the [environment reference](../reference/environment.md).

## The container image

`sysml-mcp` ships in the [`openmbee/opensysml`](https://hub.docker.com/r/openmbee/opensysml)
image beside `sysml-grpc` (the entrypoint), `sysml` and `sysml-lsp`; `--entrypoint sysml-mcp`
selects it. The image is distroless and runs as `nonroot`; the one path it writes is the standard
library's record cache under `/home/nonroot/.cache`, so a read-only root filesystem needs a volume
there (or the server runs uncached and says so once).

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

```bash
cd deploy/sysml-mcp
openssl rand -base64 32 > secrets/mcp.token
docker compose up -d
curl -sk https://localhost:8443/health
curl -sk https://localhost:8443/ready
```

The `-public-url` is what a client's `-print-config` block names; a browser-based host needs its
origin in `-allow-origin` as well.

## The systemd unit

`deploy/sysml-mcp/systemd/sysml-mcp.service` runs the binary under its own user with the
hardening systemd offers (`ProtectSystem=strict`, no new privileges, an empty capability set, a
private `/tmp`), reading every setting from `/etc/sysml-mcp/sysml-mcp.env` — the example
`sysml-mcp.env` beside it names the address, the token file, the public URL, the workspace and
the log level and the bounds, and leaves the TLS pair and the origins empty, which is unset. The unit's
`TimeoutStopSec` allows for `-shutdown-timeout` before SIGKILL; raise both together. The record
cache lands in the unit's own `/var/cache/sysml-mcp`. `deploy/sysml-mcp/README.md` lists the
installation steps.

```bash
systemctl status sysml-mcp
journalctl -u sysml-mcp -f
curl -s http://127.0.0.1:8765/ready
```

## Authentication

The server is an OAuth 2.1 resource server in its minimal form: one bearer token, issued by
whoever runs it, that every request to `/mcp` must carry; the [reference](../reference/sysml-mcp.md#transports)
has the rules. The sections below are reserved for the credentials a later release adds.

### Static bearer token

Today's `-token-file`, `-token` and `OPENSYSML_MCP_TOKEN`, as the reference describes.

### API keys

*Reserved: per-client keys issued and revoked by the operator.*

### Single sign-on (OIDC)

*Reserved: tokens issued by an identity provider, validated against its discovery document.*

### Scopes and per-principal sessions

*Reserved: which tool groups a credential reaches, and REPL sessions owned by the principal that
opened them.*

### Audit log

*Reserved: a record of who called what.*
