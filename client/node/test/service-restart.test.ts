// A child that reports its address and then exits is restarted a bounded number of times.

import assert from "node:assert/strict";
import { chmodSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

import { ServiceStartError } from "../src/core/errors.js";
import { BINARY_ENV } from "../src/node/index.js";
import { acquirePrivateService, MAX_START_ATTEMPTS, PrivateService } from "../src/node/service.js";

test(
  "a child that dies after reporting its address is not restarted forever",
  { skip: process.platform === "win32" },
  async (t) => {
    const script = join(mkdtempSync(join(tmpdir(), "opensysml-dying-")), "sysml-grpc");
    writeFileSync(script, "#!/bin/sh\necho 127.0.0.1:1\n");
    chmodSync(script, 0o700);
    const saved = process.env[BINARY_ENV];
    process.env[BINARY_ENV] = script;
    t.after(() => {
      if (saved === undefined) {
        Reflect.deleteProperty(process.env, BINARY_ENV);
      } else {
        process.env[BINARY_ENV] = saved;
      }
    });

    let started = 0;
    const startDead = async (version?: string): Promise<PrivateService> => {
      started += 1;
      const service = await PrivateService.start(version);
      while (service.alive) {
        await new Promise((resolve) => setTimeout(resolve, 5));
      }
      return service;
    };

    await assert.rejects(
      acquirePrivateService("dying-child-test", startDead),
      (error: unknown) =>
        error instanceof ServiceStartError && /exited, 3 times/.test(error.message),
    );
    assert.equal(started, MAX_START_ATTEMPTS);
  },
);
