package org.eclipse.sirius.components.collaborative.trees.palette.api;

import org.eclipse.sirius.components.core.api.IEditingContext;
import org.eclipse.sirius.components.palette.dto.Palette;
import org.eclipse.sirius.components.trees.Tree;
import org.eclipse.sirius.components.trees.TreeItem;
import org.eclipse.sirius.components.trees.description.TreeDescription;

public interface ITreeItemPaletteCustomizer {
    boolean canHandle(IEditingContext editingContext, TreeDescription treeDescription, Tree tree, TreeItem treeItem);

    Palette customize(IEditingContext editingContext, TreeDescription treeDescription, Tree tree, TreeItem treeItem,
            Palette palette);
}
