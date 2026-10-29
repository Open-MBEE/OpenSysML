- The `@openmbee/opensysml` npm client requires Node 22 or later (`engines.node >=22`); Node 20
  is no longer supported. Its Connect transport (`@connectrpc/connect-node` 2.2) and the
  optional sigstore packages behind `verifyRelease` (`@sigstore/tuf` 5, `@sigstore/verify` 4)
  dropped Node 20, which reached end of life in April 2026.
