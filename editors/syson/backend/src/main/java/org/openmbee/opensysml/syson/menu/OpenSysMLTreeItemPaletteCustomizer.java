package org.openmbee.opensysml.syson.menu;

import java.util.ArrayList;
import java.util.List;

import org.eclipse.sirius.components.collaborative.trees.dto.palette.SingleClickTreeItemTool;
import org.eclipse.sirius.components.collaborative.trees.palette.api.ITreeItemPaletteCustomizer;
import org.eclipse.sirius.components.core.api.IEditingContext;
import org.eclipse.sirius.components.core.api.IObjectSearchService;
import org.eclipse.sirius.components.palette.dto.IPaletteEntry;
import org.eclipse.sirius.components.palette.dto.Palette;
import org.eclipse.sirius.components.trees.Tree;
import org.eclipse.sirius.components.trees.TreeItem;
import org.eclipse.sirius.components.trees.description.TreeDescription;
import org.eclipse.syson.sysml.Element;
import org.springframework.stereotype.Service;

@Service
public class OpenSysMLTreeItemPaletteCustomizer implements ITreeItemPaletteCustomizer {
    public static final String TOOL_ID = "runWithOpenSysML";
    private final IObjectSearchService objectSearchService;

    public OpenSysMLTreeItemPaletteCustomizer(IObjectSearchService objectSearchService) {
        this.objectSearchService = objectSearchService;
    }

    @Override
    public boolean canHandle(IEditingContext context, TreeDescription description, Tree tree, TreeItem item) {
        return tree.getId().startsWith("explorer://")
                && objectSearchService.getObject(context, item.getId())
                        .filter(Element.class::isInstance)
                        .map(Element.class::cast)
                        .filter(element -> !element.isIsLibraryElement())
                        .isPresent();
    }

    @Override
    public Palette customize(IEditingContext context, TreeDescription description, Tree tree, TreeItem item,
            Palette palette) {
        List<IPaletteEntry> entries = new ArrayList<>(palette.paletteEntries());
        if (entries.stream().noneMatch(entry -> TOOL_ID.equals(entry.id()))) {
            entries.add(new SingleClickTreeItemTool(TOOL_ID, "Run with OpenSysML…", List.of(), false, List.of()));
        }
        return Palette.newPalette(palette.id()).quickAccessTools(palette.quickAccessTools()).paletteEntries(entries)
                .build();
    }
}
