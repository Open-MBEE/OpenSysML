# Deploying sysml-mcp

Two ways to run the MCP server as a service; the guide page
[Running sysml-mcp as a service](../../docs/guide/sysml-mcp-service.md) explains both.

- `compose.yaml` runs `sysml-mcp` from the `openmbee/opensysml` image behind Caddy, which
  terminates TLS; the token is a Docker secret read from `secrets/mcp.token`, the container's
  root filesystem is read-only, and the models come from `./models`.

  ```bash
  openssl rand -base64 32 > secrets/mcp.token
  docker compose up -d
  curl -sk https://localhost:8443/health
  curl -sk https://localhost:8443/ready
  ```

  `MCP_HOST` names the host clients use (default `localhost`), `MCP_HTTPS_PORT` the published
  port (default `8443`), `OPENSYSML_IMAGE` the image (default `openmbee/opensysml:latest`).

- `systemd/sysml-mcp.service` and `systemd/sysml-mcp.env` run the binary under its own user,
  every flag read from the environment file; the unit's header lists the steps.
