package org.eclipse.sirius.components.palette.dto;

import java.util.List;
import java.util.Objects;

public record Palette(String id, List<ITool> quickAccessTools, List<IPaletteEntry> paletteEntries) {
    public Palette {
        Objects.requireNonNull(id);
        Objects.requireNonNull(quickAccessTools);
        Objects.requireNonNull(paletteEntries);
    }

    public static Builder newPalette(String id) {
        return new Builder(id);
    }

    public static final class Builder {
        private final String id;
        private List<ITool> quickAccessTools;
        private List<IPaletteEntry> paletteEntries;

        private Builder(String id) {
            this.id = Objects.requireNonNull(id);
        }

        public Builder quickAccessTools(List<ITool> quickAccessTools) {
            this.quickAccessTools = Objects.requireNonNull(quickAccessTools);
            return this;
        }

        public Builder paletteEntries(List<IPaletteEntry> paletteEntries) {
            this.paletteEntries = Objects.requireNonNull(paletteEntries);
            return this;
        }

        public Palette build() {
            return new Palette(this.id, this.quickAccessTools, this.paletteEntries);
        }
    }
}
