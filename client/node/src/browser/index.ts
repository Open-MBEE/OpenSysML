// Browser entry point: fetch-based Connect, explicit address only. A browser
// cannot spawn a service, so there is no private-child path here.

import { createConnectTransport } from "@connectrpc/connect-web";
import { Connection } from "../core/connection.js";
import type { TransportOptions } from "../core/connection.js";
import { OpenSysMLError } from "../core/errors.js";
import { baseUrl, encodingOf, interceptors, timeoutOf } from "../core/transport.js";

export * from "../core/index.js";

/** How a browser connects: the address is required, because nothing can be started. */
export interface BrowserConnectOptions extends TransportOptions {
  address: string;
}

/**
 * Connects to a running service. The service must allow this page's exact origin
 * (`-cors-allowed-origins`) and, from an HTTPS page, must be served over TLS.
 */
export async function connect(options: BrowserConnectOptions): Promise<Connection> {
  if (options.address.trim() === "") {
    throw new OpenSysMLError(
      "a browser cannot start a sysml-grpc service, so connect() needs the address of one",
    );
  }
  const url = baseUrl(options.address);
  const encoding = encodingOf(options);
  const timeoutMs = timeoutOf(options);
  const transport = createConnectTransport({
    baseUrl: url,
    useBinaryFormat: encoding === "protobuf",
    interceptors: interceptors(options),
  });
  return Connection.open({
    transport,
    encoding,
    backend: {
      origin: url,
      // Nothing to release: the page never owned the service.
      release: () => Promise.resolve(),
    },
    timeoutMs,
  });
}
