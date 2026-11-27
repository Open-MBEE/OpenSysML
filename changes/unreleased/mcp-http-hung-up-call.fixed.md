- **`sysml-mcp` over HTTP ends a tool call whose client hung up** (`internal/frontend/mcp`): a
  `tools/call` is now cancelled when the HTTP request that carried it ends before the answer, so
  a dropped connection no longer leaves the call running to completion with nobody to answer.
