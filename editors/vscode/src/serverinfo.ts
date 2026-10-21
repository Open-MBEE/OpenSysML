/** What a server says of itself in its initialize result, when it says anything. */
export interface ServerInfo {
  name: string;
  version?: string;
}

/** declaredVersion is the version an extension's package.json declares, or "" when it declares no string. */
export function declaredVersion(packageJSON: unknown): string {
  const version = (packageJSON as { version?: unknown } | undefined)?.version;
  return typeof version === "string" ? version : "";
}

/**
 * serverBuildOf is the version the `sysml-lsp` packaged alongside an extension
 * reports: a release's `vX.Y.Z` for an extension at `X.Y.Z` (a pre-release
 * such as `X.Y.Z-rc.1` included), and a snapshot's own `nightly-<date>-<commit>`
 * tag for an extension carrying that tag as its pre-release part.
 */
export function serverBuildOf(extensionVersion: string): string {
  const night = extensionVersion.indexOf("-nightly-");
  return night < 0 ? `v${extensionVersion}` : extensionVersion.slice(night + 1);
}

/**
 * serverReport is the Output channel's line for the server a client reached: what it
 * calls itself and, when that is not the build this extension was packaged with, which
 * build was — so a stale or locally built server is told apart from the extension's.
 */
export function serverReport(extensionVersion: string, info: ServerInfo | undefined): string {
  if (info === undefined) {
    return "Connected to a server that does not report its name or version.";
  }
  const version = info.version ?? "(no version)";
  const expected = serverBuildOf(extensionVersion);
  if (info.version === expected) {
    return `Connected to ${info.name} ${version}.`;
  }
  return `Connected to ${info.name} ${version}; this extension (${extensionVersion}) was packaged with ${expected}.`;
}
