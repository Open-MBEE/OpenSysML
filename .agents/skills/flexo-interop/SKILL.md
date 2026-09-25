---
name: flexo-interop
description: How to bring up a real Flexo MMS stack (Fuseki + Layer 1 + the SysML v2 API) and measure what this project's RDF export actually survives when loaded into it — the opt-in gate `TestFlexoInterop`, its committed expectation, and the traps that make a run look like a pass or a stack failure when it is neither.
---

# Measuring this project's RDF against a running Flexo MMS

`internal/translate/export`'s RDF path writes the SysML v2 vocabulary the Flexo MMS SysML v2 service
reads. Matching that service's `Namespaces.kt` is not evidence: a predicate can be spelled
correctly and still be dropped, unreadable, or invisible to the read path. The only evidence is a
round trip through a running stack, which is what `internal/translate/interop/flexo` performs and
`internal/translate/interop/flexo/testdata/interop_expected.txt` records.

The gate is **opt-in and skips by default** (`FLEXO_INTEROP`), exactly like the corpus gates in
`tests/corpus/corpus_gate_test.go`. `go test ./...` on a machine without Docker stays green,
and a skipped run proves nothing — the skip says so on stderr.

## Bring the stack up

The published images are enough; nothing needs to be built from source.

```bash
docker compose -f /home/ubuntu/repos/flexo-mms-sysmlv2/docker-compose/docker-compose.yml up -d
```

That is Fuseki (`atomgraph/fuseki:4.6`, `:3030`), Layer 1
(`openmbee/flexo-mms-layer1-service`, `:8080`) and the SysML v2 API (`openmbee/flexo-sysmlv2`,
`:8083`). Fuseki is seeded from `docker-compose/mount/cluster.trig`, so the users, groups and
policies the JWT below relies on are already there — you do **not** need to regenerate `cluster.trig`
with the Layer 1 repository's `deploy/` script for this, and you do not need the Layer 1 repository's
own `src/test/resources/docker-compose.yml` (that one is the service's test rig: no SysML v2 API).

Layer 1's environment comes from `docker-compose/env/flexo-mms-layer1.env`
(`FLEXO_MMS_ROOT_CONTEXT=http://layer1-service`, plus the Fuseki
`FLEXO_MMS_QUERY_URL`/`FLEXO_MMS_UPDATE_URL`/`FLEXO_MMS_GRAPH_STORE_PROTOCOL_URL`), and the SysML v2
service's from `env/flexo-sysmlv2.env` (`FLEXO_HOST=layer1-service`, `FLEXO_SYSMLV2_ORG=sysmlv2`).
Both are container-internal names: from the host, use the published ports.

### Authentication

Every request to either service needs a bearer JWT signed with the stack's `JWT_SECRET`
(`env/flexo-mms-jwt.env`) and carrying `groups: ["super_admins"]`. The compose files ship a
long-lived demo token in `env/flexo-sysmlv2.env` as `FLEXO_AUTH`; for a local stack, reuse it:

```bash
source <(grep -h FLEXO_AUTH /home/ubuntu/repos/flexo-mms-sysmlv2/docker-compose/env/flexo-sysmlv2.env)
export FLEXO_INTEROP_TOKEN="${FLEXO_AUTH#Bearer }"
```

Never commit a token or paste one into a report. The harness reads it from the environment only.

## Run the gate

```bash
FLEXO_INTEROP=1 FLEXO_INTEROP_TOKEN="$FLEXO_INTEROP_TOKEN" \
  go test -count=1 ./internal/translate/interop/flexo -run TestFlexoInterop
```

About 10–20 s against a local stack. Override `FLEXO_LAYER1_URL`, `FLEXO_SYSMLV2_URL` or
`FLEXO_SYSMLV2_ORG` for a stack that is not the compose defaults.

With `FLEXO_INTEROP` set, an absent or unauthorized stack **fails**; without it, the test skips. That
asymmetry is the point: CI cannot silently stop measuring.

Re-record after an intentional encoder or fixture change, and read the diff as an
interoperability statement:

```bash
FLEXO_INTEROP=1 FLEXO_INTEROP_TOKEN="$FLEXO_INTEROP_TOKEN" \
  go test -count=1 ./internal/translate/interop/flexo -run TestFlexoInterop -update-flexo
git diff internal/translate/interop/flexo/testdata/interop_expected.txt
```

Everything else in the package (report determinism, the fixtures' coverage of the known gaps) runs
without a stack, so `go test ./internal/translate/interop/flexo` is worth running on any change to it.

## What a run does, and why each side exists

Two projects per run, both created through `POST /projects` on the SysML v2 API:

- **graph-load** — the fixture `testdata/model.sysml` converted with `export.SysMLToRDF` and
  `rdf.WriteTurtle`, `PUT` as `text/turtle` to Layer 1's
  `/orgs/{o}/repos/{r}/branches/{b}/graph`. This is the claim under test.
- **json-commit** — the same model as SysML v2 JSON changes, `POST`ed to
  `/projects/{p}/commits` from `testdata/reference_changes.json`. This is the control: it is what
  that service's *own* commit path stores, so it establishes what the read path can carry at all. If
  a property is lost on both sides, the finding is about the service, not about us.

Both sides are then read back the same way: `GET /commits`, `GET /commits/{c}/elements` (asking for
`pageSize=10`), `GET .../elements/{id}` for every written id, and `GET .../roots`. The report is
per-element and per-property; server-generated commit ids, timestamps and durations never reach it.

## Traps

**The `sysmlv2` org does not exist on a fresh cluster.** `cluster.trig` seeds users and policies, not
the org, and every `POST /projects` fails with `Org <http://layer1-service/orgs/sysmlv2> does not
exist.` until it is created. The harness `PUT`s it to Layer 1 itself (`EnsureOrg`); do the same by
hand if you are probing with curl.

**`POST /projects` must name its default branch.** Without `defaultBranch: {"@id": "main"}` the
service mints a uuid branch, and the graph endpoint's path needs a branch id you know.

**`ProjectRequest.name` is a plain string**, not a `LangString`. An object there is answered with
`400 Failed to convert request body to class ...ProjectRequest`, which reads like a bad endpoint.

**Project delete is a soft delete.** It only annotates the repo `deleted true`; the Layer 1 branch
survives, so re-creating the same project id fails with `The provided branch ... already exists.`
The harness mints a fresh id per run for that reason. To clear the accumulated projects, restart the
stack: `docker compose ... down -v && docker compose ... up -d`.

**A 404 from Layer 1's root or from an unauthenticated probe is not a stack failure.** Check the
stack with an authorized read (`GET :8083/projects`, `GET :8080/orgs` with `Accept: text/turtle`),
which is what `Reachable` does.

**Element ids must match `[a-zA-Z0-9_-]+`** (`requireValidId`). A direct read of any other id is
refused with 400 before the store is touched — the harness records that as a finding per element
rather than as an error.

**Deployed behavior differs from the source in two places** worth knowing before you read a report:
the paged listing route ignores `pageSize`/`pageAfter` and returns every subject of the branch graph,
and it therefore does not apply its own query's `sysml:elementId` filter. Elements missing
`elementId` are listed today for that reason alone.

## Driving `sysml -sync-diff` / `-sync-apply` against the stack by hand

The CLI sync (`cmd/sysml/sync.go`) shares the client above, so the same token and defaults apply
(`FLEXO_INTEROP_TOKEN`, `FLEXO_LAYER1_URL`, `FLEXO_SYSMLV2_ORG`). Things that cost time when they
are not known up front:

- **The CLI does not create the project.** Only `apply_measure.go`'s `CreateProject` does. Against a
  project id the stack has never seen, `-sync-diff`/`-sync-apply` fail on the first Layer 1 read
  (`.../branches/main/query: Not Found`). Pre-create it exactly as the harness does:
  `POST :8083/projects` with `{"@type":"Project","@id":"<id>","name":"...","defaultBranch":{"@id":"main"}}`,
  and use a fresh id per run (project deletion is soft; branches survive).
- **The endpoint argument only sets the SysML v2 URL.** The branch head and commits go there;
  the graphs themselves — the head's, the last-seen commit's — are read from Layer 1 at
  `FLEXO_LAYER1_URL` (default `http://localhost:8080`). A plaintext `http://` endpoint off this
  machine is refused (exit 2) unless `FLEXO_ALLOW_PLAIN_HTTP=1`; the compose stack on `localhost`
  needs nothing.
  So a wrong endpoint (`http://localhost:9`, or Fuseki's `:3030`) fails at the very first read
  with `read the repository: GET .../branches/main: ...` and exit 1 even when the model already
  agrees with the repository — nothing is silently accepted. An unroutable opt-in endpoint
  (`FLEXO_ALLOW_PLAIN_HTTP=1 ... http://192.0.2.10:8083`) takes ~30 s to fail with `dial tcp ...
  i/o timeout`; wrap it in `timeout 75`.
- **A no-op apply still writes `<model>.sync.json`** with the head commit it compared against
  (`nothing applied: ...; head commit <id> recorded in <state>`; a second no-op run prints no
  `recorded` and leaves the file byte-identical). Check it against `GET
  :8083/projects/<id>/branches/main` → `.head["@id"]`. A state file naming a commit the stack does
  not have fails with `read the repository at last-seen commit <id>: ...` (exit 1) and is left
  untouched. A commit is refused (exit 1, `moved from commit ... since the change set was
  computed`) if the branch head changed between the read and the write.
- **Observe commits through the API, not the CLI**: `GET :8083/projects/<id>/commits` (count and
  ids), `GET :8083/projects/<id>/commits/<commit>/elements/<elementId>` to read an element back.
  A deleted element reads back as `{"@id": ..., "@type": null}` (emptied), not 404.
- **Stage a repository-side change for a conflict** the way `MeasureApply` does: `POST
  :8083/projects/<id>/commits?branchId=main` with one `DataVersion` whose payload rewrites a
  retained id (e.g. `declaredName`). The next `-sync-apply` with a state file that names an older
  commit reports `conflict ... (changed in the repository since the last-seen commit)` and exits 1.
- Exit statuses to assert: 0 applied / nothing to change; 1 refused (unconfirmed deletes,
  conflicts, a moved head) or a repository read or write that failed; 2 usage (no token,
  `-sync-mint-ids` without `-sync-annotate`, non-http target for apply, extra positional argument).
- The full `TestFlexoInterop` + `TestFlexoInteropApply` run takes ~5 minutes against the compose
  stack (the identity and apply rounds each create a project and read every element back), not the
  10–20 s the round-trip gate alone used to take. Do not mistake the silence for a hang; check
  `docker logs --since 1m flexo-sysmlv2` for the requests it is making.

## Scope

The harness measures; it does not fix. Do not change `internal/translate/export` or `internal/translate/rdf`
encoding behavior to move a number in the expectation file, and never replace the stack with a mock —
a mocked Flexo measures our own assumptions, which is the one thing this gate exists to avoid.
