// Starts the binary with the caller's arguments and stdio, forwards the
// signals a host sends its server, and ends with the binary's status.

import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { constants } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { PACKAGE_NAME, ShimError, resolveBinary } from "./platform.js";

const here = dirname(fileURLToPath(import.meta.url));

/** The signals a host sends its server, each passed on to the binary. */
export const FORWARDED_SIGNALS = ["SIGINT", "SIGTERM", "SIGHUP"];

/** This package's version, the one the platform package must match. */
export function ownVersion() {
  return JSON.parse(readFileSync(join(here, "..", "package.json"), "utf8")).version;
}

/**
 * Runs the binary and resolves to the exit code this process should end with.
 * The binary's stdout is the protocol stream, so nothing is written there.
 *
 * @param {string[]} args the arguments to pass on
 * @param {object} [options]
 * @param {string} [options.binary] the binary to run, instead of the platform package's
 * @param {{write(text: string): unknown}} [options.stderr] where errors are reported
 * @param {NodeJS.Process} [options.host] the process whose signals are forwarded
 * @returns {Promise<number>}
 */
export function main(args, options = {}) {
  const stderr = options.stderr ?? process.stderr;
  const host = options.host ?? process;
  let binary = options.binary;
  if (binary === undefined) {
    try {
      binary = resolveBinary({ os: host.platform, cpu: host.arch, version: ownVersion() });
    } catch (error) {
      if (error instanceof ShimError) {
        stderr.write(`${error.message}\n`);
        return Promise.resolve(1);
      }
      throw error;
    }
  }
  return run(binary, args, { stderr, host });
}

/**
 * Starts binary with args on inherited stdio and resolves to the status to exit
 * with: the child's exit code, or 128 plus the signal's number when a signal
 * ended it, as a shell reports. Signals the host process receives meanwhile
 * are passed on to the child.
 */
export function run(binary, args, { stderr, host }) {
  return new Promise((resolve) => {
    const child = spawn(binary, args, { stdio: "inherit", windowsHide: true });
    const listeners = FORWARDED_SIGNALS.map((signal) => {
      const listener = () => {
        if (child.exitCode === null && child.signalCode === null) {
          child.kill(signal);
        }
      };
      host.on(signal, listener);
      return [signal, listener];
    });
    const done = (status) => {
      for (const [signal, listener] of listeners) {
        host.removeListener(signal, listener);
      }
      resolve(status);
    };
    child.on("error", (error) => {
      stderr.write(`${describeSpawnError(binary, error)}\n`);
      done(1);
    });
    child.on("exit", (code, signal) => {
      done(code ?? signalStatus(signal));
    });
  });
}

/** The status a process ended by a signal is reported with: 128 plus the signal's number. */
export function signalStatus(signal) {
  const number = constants.signals[signal];
  return number === undefined ? 1 : 128 + number;
}

function describeSpawnError(binary, error) {
  if (error.code === "ENOENT" || error.code === "EACCES") {
    return (
      `${PACKAGE_NAME}: ${binary} cannot be started (${error.code}); the platform package ` +
      `is incomplete. Reinstall ${PACKAGE_NAME}.`
    );
  }
  return `${PACKAGE_NAME}: could not start ${binary}: ${error.message}`;
}
