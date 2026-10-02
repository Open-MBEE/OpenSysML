---
name: testing-node-browser-client
description: How to test the @openmbee/opensysml browser entry point (client/node/src/browser) against a real sysml-grpc service — CORS-allowed origins, observing requests without altering them, decoding captured bodies, protobuf versus JSON encodings, and differential comparison with the Python client.
---

# Testing the Node client's browser entry point

## Setup

- Read the browser section of `client/node/README.md` and `docs/reference/node-api.md` (In a
  browser), and `client/node/test/browser.test.ts`, which starts the service the way a page needs it.
- Build the service (`make build-grpc`) and start it with the page's exact origin allowed:
  `bin/sysml-grpc -port 0 -health-port 0 -report-address -cors-allowed-origins <origin>`. The first
  stdout line is the address. Never allow `*`; an HTTPS page also needs `-tls-cert`/`-tls-key`.
- Bundle a harness page importing `@openmbee/opensysml/browser` outside the checkout and serve it from
  the allowed origin. `connect({ address })` is the only path: a browser cannot start a service.

## Shared-core browser validation

- Wrap the browser's `fetch` only to observe and forward the original request and response
  unchanged. Counting observed service calls distinguishes a client-side refusal from a similarly
  typed backend error; assert zero requests for invalid inputs.
- Decode captured unary request bodies with the generated schema and protobuf `fromBinary`/`fromJson`.
  That verifies `strictConformance` and collection union kinds while the real request still runs.
  Never substitute responses.
- Verify collection conversion with a real Echo calculation, not only `toValue`: check both the
  outgoing sequence/set union and the returned values.
- Read the public result types before writing assertions. Verification methods can return a wrapper
  with `.verdict.standing`, while `CalcResult` has `.standing`.
- Run every check under both protobuf and JSON (`connect({ encoding: "json" })`). Record the concrete
  error class, code, `instanceof` result and elapsed time rather than assuming browser `fetch` maps
  failures the way Node gRPC does; an unreachable service or CORS refusal arrives as `UNKNOWN`.

## Differential options and baseline preservation

- When reusing saved Python output, verify every baseline file is unchanged.
- Compare option semantics, not just spelling: Python `strict` raises on parsing diagnostics, while
  Node's `strict` is a deprecated alias of `strictConformance`. That intentional difference can change
  hashes for otherwise identical source. Report it, or agree on equivalent inputs before regenerating
  baselines; do not quietly remove or normalize the hash field.
- Keep approved differences separate from newly acknowledged intentional exceptions. A prior finding
  is not automatically an approved difference.

### Devin Secrets Needed

None for a local unauthenticated service and an external harness.
