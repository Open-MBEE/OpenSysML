- The `@openmbee/opensysml` npm client requires Node 22.22.2 or later (`engines.node >=22.22.2`);
  Node 20 is no longer supported. Its Connect transport (`@connectrpc/connect-node` 2.2) dropped
  Node 20, which reached end of life in April 2026, and the optional sigstore packages behind
  `verifyRelease` (`@sigstore/tuf` 5, `@sigstore/verify` 4) require 22.22.2 within the 22 line —
  on an older 22, npm skips them as optional and `verifyRelease` is unavailable.
