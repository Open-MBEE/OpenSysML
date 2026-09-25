// A local stand-in for a GitHub release, so the downloader can be tested offline.

import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";

/** What a served release publishes. An absent asset is answered with a 404. */
export interface Release {
  version: string;
  repo: string;
  /** Name of the binary asset, and of the `.sha256` sidecar beside it. */
  asset: string;
  binary?: Buffer | undefined;
  /** Digest served in the sidecar, which need not be the binary's. */
  checksum?: string | undefined;
  manifest?: Buffer | undefined;
  bundle?: Buffer | undefined;
  /** Tag the releases API reports as the latest release. */
  latest?: string | undefined;
}

/** A running release, and the requests it has answered. */
export interface ServedRelease {
  /** Base URL to download release assets from. */
  url: string;
  /** Base URL of the releases API. */
  apiUrl: string;
  /** Paths requested, in order, whether they were served or not. */
  requested: string[];
  close: () => Promise<void>;
}

/** Serve a release on a loopback port until the test closes it. */
export async function serveRelease(release: Release): Promise<ServedRelease> {
  const requested: string[] = [];
  const server = createServer((request, response) => {
    const path = request.url ?? "";
    requested.push(path);
    const body = published(release, path);
    if (body === undefined) {
      response.writeHead(404, { "content-type": "text/plain" });
      response.end("Not Found");
      return;
    }
    response.writeHead(200, { "content-type": "application/octet-stream" });
    response.end(body);
  });

  await new Promise<void>((resolve) => {
    server.listen(0, "127.0.0.1", resolve);
  });
  const url = `http://127.0.0.1:${String((server.address() as AddressInfo).port)}`;
  return {
    url,
    apiUrl: url,
    requested,
    close: () => close(server),
  };
}

function published(release: Release, path: string): Buffer | undefined {
  if (path === `/repos/${release.repo}/releases/latest`) {
    return release.latest === undefined
      ? undefined
      : Buffer.from(JSON.stringify({ tag_name: release.latest }));
  }

  const prefix = `/${release.repo}/releases/download/${release.version}/`;
  if (!path.startsWith(prefix)) {
    return undefined;
  }
  const asset = path.slice(prefix.length);
  if (asset === release.asset) {
    return release.binary;
  }
  if (asset === `${release.asset}.sha256`) {
    return release.checksum === undefined
      ? undefined
      : Buffer.from(`${release.checksum}  ${release.asset}\n`);
  }
  if (asset === "SHA256SUMS.txt") {
    return release.manifest;
  }
  if (asset === "SHA256SUMS.txt.bundle") {
    return release.bundle;
  }
  return undefined;
}

async function close(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    server.close((error) => {
      if (error === undefined) {
        resolve();
      } else {
        reject(error);
      }
    });
  });
}
