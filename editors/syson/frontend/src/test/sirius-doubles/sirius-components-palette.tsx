import type React from 'react';
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
  component: React.ComponentType<PaletteToolOverriddenContributionComponentProps>;
}

export const paletteToolOverrideExtensionPoint: DataExtensionPoint<PaletteToolOverriddenContributionProps[]> = {
  identifier: 'palette#toolOverride',
  fallback: [],
};

export interface MatchResult {
  matches: boolean;
}

export const fuzzyMatch = (candidate: string, searchTerm: string): MatchResult => ({
  matches: candidate.toLowerCase().includes(searchTerm.toLowerCase()),
});

export const ToolListItemText = ({ label }: { label: string; searchedValue: string | null }) => <span>{label}</span>;
