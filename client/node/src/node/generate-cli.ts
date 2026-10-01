#!/usr/bin/env node
// The opensysml-generate bin: generate typed TypeScript classes from a model.

import { main } from "./generate.js";

main(process.argv.slice(2))
  .then((code) => {
    process.exitCode = code;
  })
  .catch((error: unknown) => {
    console.error(`error: ${(error as Error).message}`);
    process.exitCode = 2;
  });
