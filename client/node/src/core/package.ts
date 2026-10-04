/** The npm name this client is published under; package.json's "name" must match it. */
export const PACKAGE_NAME = "@openmbee/opensysml";

/** The optional npm package that carries the combined WebAssembly module. */
export const WASM_PACKAGE = `${PACKAGE_NAME}-wasm`;

/** Prefix of the per-platform packages that carry sysml-grpc: `${prefix}<os>-<cpu>`. */
export const PLATFORM_PACKAGE_PREFIX = `${PACKAGE_NAME}-sysml-grpc-`;
