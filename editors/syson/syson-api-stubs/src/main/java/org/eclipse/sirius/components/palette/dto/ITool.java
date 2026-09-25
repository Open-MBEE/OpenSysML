package org.eclipse.sirius.components.palette.dto;

import java.util.List;

public interface ITool extends IPaletteEntry {
    String label();

    List<String> iconURL();
}
