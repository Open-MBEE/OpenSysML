import { ExtensionRegistry as SiriusExtensionRegistry } from '@eclipse-sirius/sirius-components-core';
import {
  GQLTool,
  PaletteToolOverriddenContributionProps,
  paletteToolOverrideExtensionPoint,
} from '@eclipse-sirius/sirius-components-palette';
import {
  GQLTreeItemContextMenuEntry,
  TREE_REPRESENTATION_KIND,
  TreeItemContextMenuOverrideContribution,
  treeItemContextMenuEntryOverrideExtensionPoint,
} from '@eclipse-sirius/sirius-components-trees';
import { RUN_WITH_OPENSYSML_TOOL_ID } from '../constants';
import { RunWithOpenSysMLMenuContribution } from '../extension/RunWithOpenSysMLMenuContribution';
import { RunWithOpenSysMLPaletteContribution } from '../extension/RunWithOpenSysMLPaletteContribution';

interface ExtensionRegistry {
  putData<P>(extensionPoint: { identifier: string; fallback: P }, extension: { identifier: string; data: P }): void;
  getData<P>(extensionPoint: { identifier: string; fallback: P }): { identifier: string; data: P } | null;
}

// The explorer shows the server's tool either in its context menu or, since SysON 2026.9, in its palette.
const menuContribution: TreeItemContextMenuOverrideContribution = {
  canHandle: (entry: GQLTreeItemContextMenuEntry) => entry.id === RUN_WITH_OPENSYSML_TOOL_ID,
  // The menu uses only four of Sirius' props; the contribution type wants the full set.
  component: RunWithOpenSysMLMenuContribution as unknown as TreeItemContextMenuOverrideContribution['component'],
};

const paletteContribution: PaletteToolOverriddenContributionProps = {
  canHandle: (representationKind: string, tool: GQLTool) =>
    representationKind === TREE_REPRESENTATION_KIND && tool.id === RUN_WITH_OPENSYSML_TOOL_ID,
  component: RunWithOpenSysMLPaletteContribution,
};

const append = <P>(
  registry: ExtensionRegistry,
  extensionPoint: { identifier: string; fallback: P[] },
  fallbackIdentifier: string,
  contribution: P
): void => {
  const existing = registry.getData(extensionPoint);
  // SysON's registry replaces data at an extension point, so append without dropping its contributions.
  registry.putData(extensionPoint, {
    identifier: existing?.identifier ?? fallbackIdentifier,
    data: [...(existing?.data ?? []), contribution],
  });
};

export const addOpenSysMLContributions = (registry: ExtensionRegistry): void => {
  append(
    registry,
    treeItemContextMenuEntryOverrideExtensionPoint,
    'opensysml_treeItemContextMenuEntryOverride',
    menuContribution
  );
  append(registry, paletteToolOverrideExtensionPoint, 'opensysml_paletteToolOverride', paletteContribution);
};

export const opensysmlExtensionRegistry: ExtensionRegistry = new SiriusExtensionRegistry();
addOpenSysMLContributions(opensysmlExtensionRegistry);
