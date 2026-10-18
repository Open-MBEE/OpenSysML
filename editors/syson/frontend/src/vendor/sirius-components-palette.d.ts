declare module '@eclipse-sirius/sirius-components-palette' {
  import type { DataExtensionPoint } from '@eclipse-sirius/sirius-components-core';

  export interface GQLTool {
    id: string;
    label: string;
    iconURL: string[];
    __typename: string;
  }

  export interface PaletteToolOverriddenContributionComponentProps {
    representationElementIds: string[];
    tool: GQLTool;
    onInvoked: () => void;
    asLastToolUsed: boolean;
    isToolInPalette: boolean;
    searchedValue: string | null;
  }

  export interface PaletteToolOverriddenContributionProps {
    canHandle: (representationKind: string, tool: GQLTool) => boolean;
    component: import('react').ComponentType<PaletteToolOverriddenContributionComponentProps>;
  }

  export const paletteToolOverrideExtensionPoint: DataExtensionPoint<PaletteToolOverriddenContributionProps[]>;

  export interface MatchResult {
    matches: boolean;
  }

  export const fuzzyMatch: (candidate: string, searchTerm: string) => MatchResult;

  export const ToolListItemText: (props: { label: string; searchedValue: string | null }) => JSX.Element;
}
