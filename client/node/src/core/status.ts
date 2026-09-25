// The client boundary for the service's statuses: a failed call is translated
// into this client's error hierarchy, keeping the ConnectError as its cause.

import { Code, ConnectError } from "@connectrpc/connect";
import {
  InvalidRequestError,
  ModelFileNotFoundError,
  ModelNotFoundError,
  OpenSysMLError,
  ServiceError,
  ServiceTimeoutError,
  SymbolNotFoundError,
  UnsupportedOperationError,
} from "./errors.js";

/** A status code as the service's conformance scenarios spell it. */
const STATUS_NAMES = new Map<Code, string>([
  [Code.Canceled, "CANCELLED"],
  [Code.Unknown, "UNKNOWN"],
  [Code.InvalidArgument, "INVALID_ARGUMENT"],
  [Code.DeadlineExceeded, "DEADLINE_EXCEEDED"],
  [Code.NotFound, "NOT_FOUND"],
  [Code.AlreadyExists, "ALREADY_EXISTS"],
  [Code.PermissionDenied, "PERMISSION_DENIED"],
  [Code.ResourceExhausted, "RESOURCE_EXHAUSTED"],
  [Code.FailedPrecondition, "FAILED_PRECONDITION"],
  [Code.Aborted, "ABORTED"],
  [Code.OutOfRange, "OUT_OF_RANGE"],
  [Code.Unimplemented, "UNIMPLEMENTED"],
  [Code.Internal, "INTERNAL"],
  [Code.Unavailable, "UNAVAILABLE"],
  [Code.DataLoss, "DATA_LOSS"],
  [Code.Unauthenticated, "UNAUTHENTICATED"],
]);

/** The name of a status code, as a scenario and an error message spell it. */
export function statusName(code: Code): string {
  return STATUS_NAMES.get(code) ?? String(code);
}

type ServiceErrorClass = new (
  message: string,
  options?: { cause?: unknown; code?: string },
) => ServiceError;

// Statuses that name a distinct failure. Anything else stays a ServiceError, so
// a status this client has never seen still arrives inside the hierarchy.
const CODE_ERRORS = new Map<Code, ServiceErrorClass>([
  [Code.InvalidArgument, InvalidRequestError],
  [Code.FailedPrecondition, InvalidRequestError],
  [Code.OutOfRange, InvalidRequestError],
  [Code.DeadlineExceeded, ServiceTimeoutError],
  [Code.Canceled, ServiceTimeoutError],
  [Code.Unimplemented, UnsupportedOperationError],
]);

/**
 * What a NOT_FOUND names when the service's message does not say. The call site
 * knows what it asked about: a path, or a model held under a hash.
 */
export type NotFoundSubject = "model" | "file";

/** Translates a failed call into the error for its status. */
export function fromRpcError(error: unknown, notFound: NotFoundSubject = "model"): OpenSysMLError {
  // A capability refusal or a closed connection is already this client's own.
  if (error instanceof OpenSysMLError) {
    return error;
  }
  const connectError = ConnectError.from(error);
  const status = statusName(connectError.code);
  const message =
    connectError.rawMessage === ""
      ? `the sysml-grpc service failed the call with ${status}`
      : connectError.rawMessage;
  if (connectError.code === Code.NotFound) {
    return notFoundError(message, status, notFound, connectError);
  }
  const cls = CODE_ERRORS.get(connectError.code) ?? ServiceError;
  const described =
    connectError.code === Code.Unavailable ? `sysml-grpc service unavailable: ${message}` : message;
  return new cls(described, { cause: connectError, code: status });
}

/** Translates a failed handshake, which says which service would not answer. */
export function fromHandshakeError(error: unknown, origin: string): ServiceError {
  const connectError = ConnectError.from(error);
  const cls = CODE_ERRORS.get(connectError.code) ?? ServiceError;
  return new cls(`the sysml-grpc service at ${origin} did not answer: ${connectError.message}`, {
    cause: connectError,
    code: statusName(connectError.code),
  });
}

/** Awaits a call, translating whatever status it fails with. */
export async function callRpc<T>(call: Promise<T>, notFound: NotFoundSubject = "model"): Promise<T> {
  try {
    return await call;
  } catch (error) {
    throw fromRpcError(error, notFound);
  }
}

// The service answers NOT_FOUND for a source file it cannot read, a model hash
// it no longer holds and a symbol a model does not declare; it says which.
function notFoundError(
  message: string,
  status: string,
  notFound: NotFoundSubject,
  cause: ConnectError,
): OpenSysMLError {
  const lowered = message.toLowerCase();
  if (lowered.includes("file not found") || lowered.includes("no such file")) {
    return new ModelFileNotFoundError(message, { cause, code: status });
  }
  if (lowered.includes("model not found")) {
    return new ModelNotFoundError(message, { cause, code: status });
  }
  if (lowered.includes("symbol not found")) {
    return new SymbolNotFoundError(symbolNameIn(message));
  }
  const cls = notFound === "file" ? ModelFileNotFoundError : ModelNotFoundError;
  return new cls(message, { cause, code: status });
}

function symbolNameIn(message: string): string {
  const marker = "symbol not found: ";
  const at = message.lastIndexOf(marker);
  return at === -1 ? message : message.slice(at + marker.length);
}
