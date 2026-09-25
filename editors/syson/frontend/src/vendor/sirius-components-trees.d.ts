declare module '@eclipse-sirius/sirius-components-trees' {
  import type { DataExtensionPoint, GQLStyledString } from '@eclipse-sirius/sirius-components-core';

  export interface GQLTreeItem {
    id: string;
    label: GQLStyledString;
    kind: string;
    iconURL: [string];
    hasChildren: boolean;
    children: GQLTreeItem[];
    expanded: boolean;
    editable: boolean;
    deletable: boolean;
    selectable: boolean;
  }

  export interface GQLKeyBinding {
    isCtrl: boolean;
    isMeta: boolean;
    isAlt: boolean;
    key: string;
  }

  export interface GQLTreeItemContextMenuEntry {
    id: string;
    label: string;
    iconURL: string[];
    keyBindings: GQLKeyBinding[];
    __typename: string;
  }

  export interface TreeItemContextMenuComponentProps {
    editingContextId: string;
    treeId: string;
    item: GQLTreeItem;
    entry: GQLTreeItemContextMenuEntry | null;
    readOnly: boolean;
    expandItem: () => void;
    selectTreeItems: (selectedTreeItemIds: string[]) => void;
    onExpandedElementChange: (expanded: string[], maxDepth: number) => void;
    onClose: () => void;
    key: string;
    expanded: string[];
    maxDepth: number;
    selectedTreeItemIds: string[];
  }

  export interface TreeItemContextMenuOverrideContribution {
    canHandle: (entry: GQLTreeItemContextMenuEntry) => boolean;
    component: import('react').ComponentType<TreeItemContextMenuComponentProps>;
  }

  export const treeItemContextMenuEntryOverrideExtensionPoint: DataExtensionPoint<
    TreeItemContextMenuOverrideContribution[]
  >;
}
