import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import MenuItem from '@mui/material/MenuItem';
import { forwardRef, useState } from 'react';
import type { TreeItemContextMenuComponentProps } from '@eclipse-sirius/sirius-components-trees';
import { RunWithOpenSysMLDialog } from '../dialog/RunWithOpenSysMLDialog';

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
