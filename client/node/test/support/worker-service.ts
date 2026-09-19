// Test helper: connects from a worker thread and reports the pid of the child
// that thread started, then closes.

import { parentPort } from "node:worker_threads";
import { connect, currentPrivateService } from "../../src/node/index.js";

const connection = await connect();
const service = currentPrivateService();
parentPort?.postMessage({ pid: service?.pid });
await connection.close();
