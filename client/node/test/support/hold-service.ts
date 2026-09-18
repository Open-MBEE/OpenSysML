// Test helper: connects, reports the child's pid, and then either waits to be
// killed or returns without closing. Run as `hold-service.js wait|return`.

import { connect, currentPrivateService } from "../../src/node/index.js";
import { useServiceBinary } from "./service.js";

useServiceBinary();
const mode = process.argv[2] ?? "wait";
const connection = await connect();
const service = currentPrivateService();
process.stdout.write(`${JSON.stringify({ pid: service?.pid, address: connection.info.origin })}\n`);
if (mode === "wait") {
  // Nothing else to do: the client must not let this process exit on its own.
  setInterval(() => {
    // keeps the event loop alive until the parent kills this process
  }, 1000);
}
