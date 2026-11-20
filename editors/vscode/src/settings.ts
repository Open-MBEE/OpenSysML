/** The settings the server reads, as the client sends them in initializationOptions
 * and workspace/didChangeConfiguration. */
export interface ServerSettings {
  sysml: { keepLineBreaks: boolean; hoverMemberLimit: number; hoverInheritedMembers: boolean };
}

/** The configuration sections the server's settings live under. */
export const SERVER_SETTING_SECTIONS = ["opensysml.format", "opensysml.hover"];

/** Reads the server's settings out of the `opensysml` configuration. */
export function serverSettings(config: { get<T>(key: string, defaultValue: T): T }): ServerSettings {
  return {
    sysml: {
      keepLineBreaks: config.get<boolean>("format.keepLineBreaks", false),
      hoverMemberLimit: config.get<number>("hover.memberLimit", 10),
      hoverInheritedMembers: config.get<boolean>("hover.inheritedMembers", false),
    },
  };
}
