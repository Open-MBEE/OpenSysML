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
  every flag read from the environment file. Install the binary at `/usr/local/bin/sysml-mcp`, then:

  ```bash
  useradd --system --home /var/lib/sysml-mcp --create-home sysml-mcp
  install -d -o sysml-mcp -g sysml-mcp -m 0750 /etc/sysml-mcp
  openssl rand -base64 32 > /etc/sysml-mcp/mcp.token
  chmod 0640 /etc/sysml-mcp/mcp.token && chown sysml-mcp /etc/sysml-mcp/mcp.token
  cp systemd/sysml-mcp.service /etc/systemd/system/ && cp systemd/sysml-mcp.env /etc/sysml-mcp/
  systemctl enable --now sysml-mcp
  ```
