- **`sysml-mcp` accepts the access tokens of an OpenID provider** (`internal/frontend/guard/oidc`):
  the provider is read from its discovery document, its JWKS fetched and cached, and a JWT is
  accepted when its signature, `iss`, `aud` (the MCP resource URL, RFC 8707), `exp` and `nbf`
  verify, its scopes read from the `scope` or `scp` claim (or a configured claim, and a roles map);
  an expired token is refused as `invalid_token` with the RFC 6750 description, a token the
  provider cannot be reached to verify as 503. Opaque tokens are introspected (RFC 7662) when an
  endpoint and client credential are configured, each active answer kept until its `exp`; a
  bearer token that is not a JWT is otherwise left to the next authenticator of the chain. The
  protected resource metadata names the issuer in `authorization_servers` and the scopes in
  `scopes_supported`, so an MCP host finds the provider and runs Authorization Code + PKCE.
