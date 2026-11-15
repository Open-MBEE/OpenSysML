- **`-oidc-issuer URL`** has `sysml-mcp` (and `sysml-grpc`, which shares the authenticator flags)
  accept the access tokens of an OpenID provider: `-oidc-audience` names what their `aud` must
  hold (for `sysml-mcp`, the `-public-url` endpoint by default), `-oidc-scope-claim` the claim
  their scopes are read from, and `-oidc-introspect` with `-oidc-client-file` the RFC 7662
  endpoint and client that opaque tokens are checked through. The protected resource metadata
  then names the provider and the tools' scopes, so an MCP host runs Authorization Code + PKCE
  against it; `docs/reference/sysml-mcp.md` § Single sign-on walks through Keycloak with the
  realm under `deploy/keycloak/` and notes Entra ID, Okta and Ping.
