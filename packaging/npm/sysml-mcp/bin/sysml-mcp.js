#!/usr/bin/env node
// The `sysml-mcp` command of @openmbee/sysml-mcp: starts the release binary the
// platform package for this machine carries, with this process's arguments and
// stdio. Nothing is written to stdout here; it belongs to the protocol.
import { main } from "../lib/main.js";

process.exitCode = await main(process.argv.slice(2));
