import { ExtensionRegistry as SiriusExtensionRegistry } from '@eclipse-sirius/sirius-components-core';
import {
  GQLTreeItemContextMenuEntry,
  TreeItemContextMenuOverrideContribution,
  treeItemContextMenuEntryOverrideExtensionPoint,
} from '@eclipse-sirius/sirius-components-trees';
import { RUN_WITH_OPENSYSML_TOOL_ID } from '../constants';
import { RunWithOpenSysMLMenuContribution } from '../extension/RunWithOpenSysMLMenuContribution';

interface ExtensionRegistry {
  putData<P>(extensionPoint: { identifier: string; fallback: P }, extension: { identifier: string; data: P }): void;
  getData<P>(extensionPoint: { identifier: string; fallback: P }): { identifier: string; data: P } | null;
}

const contribution: TreeItemContextMenuOverrideContribution = {
  canHandle: (entry: GQLTreeItemContextMenuEntry) => entry.id === RUN_WITH_OPENSYSML_TOOL_ID,
  // The menu uses only four of Sirius' props; the contribution type wants the full set.
  component: RunWithOpenSysMLMenuContribution as unknown as TreeItemContextMenuOverrideContribution['component'],
};

export const opensysmlExtensionRegistry: ExtensionRegistry = new SiriusExtensionRegistry();

opensysmlExtensionRegistry.putData(treeItemContextMenuEntryOverrideExtensionPoint, {
  identifier: 'opensysml_treeItemContextMenuEntryOverride',
  data: [contribution],
});

export const addOpenSysMLContributions = (registry: ExtensionRegistry): void => {
  const existing = registry.getData(treeItemContextMenuEntryOverrideExtensionPoint);
  // SysON's registry replaces data at this extension point, so append without dropping its contributions.
  registry.putData(treeItemContextMenuEntryOverrideExtensionPoint, {
    identifier: existing?.identifier ?? 'opensysml_treeItemContextMenuEntryOverride',
    data: [...(existing?.data ?? []), contribution],
  });
};
