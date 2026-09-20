package org.eclipse.sirius.components.core.api;

import java.util.UUID;

import org.eclipse.sirius.components.events.ICause;

public interface IInput extends ICause {
    UUID id();
}
