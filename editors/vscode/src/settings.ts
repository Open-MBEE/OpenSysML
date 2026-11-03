/** The settings the server reads, as the client sends them in initializationOptions
 * and workspace/didChangeConfiguration. */
export interface ServerSettings {
  sysml: { keepLineBreaks: boolean };
}

/** The configuration section the server's settings live under. */
export const FORMAT_SECTION = "opensysml.format";

/** Reads the server's settings out of the `opensysml` configuration. */
export function serverSettings(config: { get<T>(key: string, defaultValue: T): T }): ServerSettings {
  return { sysml: { keepLineBreaks: config.get<boolean>("format.keepLineBreaks", false) } };
}
