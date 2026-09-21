package org.eclipse.sirius.components.collaborative.trees.dto.palette;

import java.util.List;

import org.eclipse.sirius.components.collaborative.dto.KeyBinding;
import org.eclipse.sirius.components.palette.dto.ITool;

public record SingleClickTreeItemTool(String id, String label, List<String> iconURL, boolean withImpactAnalysis,
        List<KeyBinding> keyBindings) implements ITool {
}
