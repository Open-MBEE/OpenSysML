import { ExtensionRegistry } from '@eclipse-sirius/sirius-components-core';
import {
  GQLTreeItemContextMenuEntry,
  TreeItemContextMenuOverrideContribution,
  treeItemContextMenuEntryOverrideExtensionPoint,
} from '@eclipse-sirius/sirius-components-trees';
import { RUN_WITH_OPENSYSML_TOOL_ID } from '../constants';
import { RunWithOpenSysMLMenuContribution } from '../extension/RunWithOpenSysMLMenuContribution';

const contribution: TreeItemContextMenuOverrideContribution = {
  canHandle: (entry: GQLTreeItemContextMenuEntry) => entry.id === RUN_WITH_OPENSYSML_TOOL_ID,
  component: RunWithOpenSysMLMenuContribution,
};

export const opensysmlExtensionRegistry = new ExtensionRegistry();

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
