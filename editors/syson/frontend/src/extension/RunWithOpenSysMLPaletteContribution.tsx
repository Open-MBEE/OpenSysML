import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import PublishIcon from '@mui/icons-material/Publish';
import ListItemIcon from '@mui/material/ListItemIcon';
import MenuItem from '@mui/material/MenuItem';
import {
  fuzzyMatch,
  ToolListItemText,
  type PaletteToolOverriddenContributionComponentProps,
} from '@eclipse-sirius/sirius-components-palette';
import { TreePaletteContext } from '@eclipse-sirius/sirius-components-trees';
import { forwardRef, useContext, useState } from 'react';
import { PUBLISH_WITH_OPENSYSML_TOOL_ID } from '../constants';
import { PublishWithOpenSysMLDialog } from '../dialog/PublishWithOpenSysMLDialog';
import { RunWithOpenSysMLDialog } from '../dialog/RunWithOpenSysMLDialog';

/** The explorer palette's rendering of the run and publish tools; the palette replaces the context menu in SysON 2026.9. */
export const RunWithOpenSysMLPaletteContribution = forwardRef<
  HTMLLIElement,
  PaletteToolOverriddenContributionComponentProps
>(({ tool, onInvoked, searchedValue }, ref) => {
  const { editingContextId, treeId, item, readOnly, onClose } = useContext(TreePaletteContext);
  const [open, setOpen] = useState(false);
  if (!treeId.startsWith('explorer://') || readOnly || item === null) {
    return null;
  }
  if (searchedValue && !fuzzyMatch(tool.label, searchedValue).matches) {
    return null;
  }

  const elementLabel = item.label.styledStringFragments.map((fragment) => fragment.text).join('');
  const close = (): void => {
    setOpen(false);
    onClose();
  };

  const publishes = tool.id === PUBLISH_WITH_OPENSYSML_TOOL_ID;

  return (
    <>
      <MenuItem
        ref={ref}
        data-testid={publishes ? 'publish-with-opensysml-tool' : 'run-with-opensysml-tool'}
        onClick={() => {
          onInvoked();
          setOpen(true);
        }}>
        <ListItemIcon>{publishes ? <PublishIcon fontSize="small" /> : <PlayArrowIcon fontSize="small" />}</ListItemIcon>
        <ToolListItemText label={tool.label} searchedValue={searchedValue} />
      </MenuItem>
      {open && publishes && <PublishWithOpenSysMLDialog editingContextId={editingContextId} onClose={close} />}
      {open && !publishes && (
        <RunWithOpenSysMLDialog
          editingContextId={editingContextId}
          objectId={item.id}
          elementLabel={elementLabel}
          onClose={close}
        />
      )}
    </>
  );
});
