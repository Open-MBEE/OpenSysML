package org.eclipse.syson.sysml.metamodel.services.textual;

import java.util.Objects;

import org.eclipse.syson.sysml.metamodel.services.textual.utils.INameDeresolver;

public record SysMLSerializingOptions(String lineSeparator, String indentation, INameDeresolver nameDeresolver,
        boolean needEscapeCharacter) {
    public SysMLSerializingOptions {
        Objects.requireNonNull(lineSeparator);
        Objects.requireNonNull(indentation);
        Objects.requireNonNull(nameDeresolver);
    }

    public static final class Builder {
        private String lineSeparator;
        private String indentation;
        private INameDeresolver nameDeresolver;
        private boolean needEscapeCharacter;

        public Builder lineSeparator(String lineSeparator) {
            this.lineSeparator = Objects.requireNonNull(lineSeparator);
            return this;
        }

        public Builder indentation(String indentation) {
            this.indentation = Objects.requireNonNull(indentation);
            return this;
        }

        public Builder nameDeresolver(INameDeresolver nameDeresolver) {
            this.nameDeresolver = Objects.requireNonNull(nameDeresolver);
            return this;
        }

        public Builder needEscapeCharacter(boolean needEscapeCharacter) {
            this.needEscapeCharacter = needEscapeCharacter;
            return this;
        }

        public SysMLSerializingOptions build() {
            return new SysMLSerializingOptions(this.lineSeparator, this.indentation, this.nameDeresolver,
                    this.needEscapeCharacter);
        }
    }
}
