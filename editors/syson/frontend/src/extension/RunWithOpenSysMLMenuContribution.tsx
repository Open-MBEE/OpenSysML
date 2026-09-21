import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import MenuItem from '@mui/material/MenuItem';
import { forwardRef, useState } from 'react';
import { RunWithOpenSysMLDialog } from '../dialog/RunWithOpenSysMLDialog';

type GQLTreeItem = {
  id: string;
  label: {
    styledStringFragments: { text: string }[];
  };
  kind: string;
  iconURL: [string];
  hasChildren: boolean;
  children: GQLTreeItem[];
  expanded: boolean;
  editable: boolean;
  deletable: boolean;
  selectable: boolean;
};

type TreeItemContextMenuComponentProps = {
  editingContextId: string;
  treeId: string;
  item: GQLTreeItem;
  entry: {
    id: string;
    label: string;
    iconURL: string[];
    keyBindings: {
      isCtrl: boolean;
      isMeta: boolean;
      isAlt: boolean;
      key: string;
    }[];
    __typename: string;
  } | null;
  readOnly: boolean;
  expandItem: () => void;
  selectTreeItems: (selectedTreeItemIds: string[]) => void;
  onExpandedElementChange: (expanded: string[], maxDepth: number) => void;
  onClose: () => void;
  key: string;
  expanded: string[];
  maxDepth: number;
  selectedTreeItemIds: string[];
};

export const RunWithOpenSysMLMenuContribution = forwardRef<HTMLLIElement, TreeItemContextMenuComponentProps>(
  ({ editingContextId, treeId, item, onClose }, ref) => {
    const [open, setOpen] = useState(false);
    if (!treeId.startsWith('explorer://')) {
      return null;
    }

    const elementLabel = item.label.styledStringFragments.map((fragment) => fragment.text).join('');
    const close = (): void => {
      setOpen(false);
      onClose();
    };

    return (
      <>
        <MenuItem ref={ref} data-testid="run-with-opensysml-menu" onClick={() => setOpen(true)} disabled={false}>
          <ListItemIcon>
            <PlayArrowIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary="Run with OpenSysML…" />
        </MenuItem>
        {open && (
          <RunWithOpenSysMLDialog
            editingContextId={editingContextId}
            objectId={item.id}
            elementLabel={elementLabel}
            onClose={close}
          />
        )}
      </>
    );
  }
);
