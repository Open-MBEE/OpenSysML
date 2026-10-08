#!/usr/bin/env node
// The opensysml-generate bin: generate typed TypeScript classes from a model.

import { main } from "./generate.js";

try {
  process.exitCode = await main(process.argv.slice(2));
} catch (error: unknown) {
  console.error(`error: ${(error as Error).message}`);
  process.exitCode = 2;
}
