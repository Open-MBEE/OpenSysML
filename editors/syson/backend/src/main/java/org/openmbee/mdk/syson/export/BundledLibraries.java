package org.openmbee.mdk.syson.export;

import java.util.Optional;
import java.util.Set;

// Answers whether the OpenSysML service already provides a library package of this name, and
// with which declarations: the names of every element under it, relative to it, at any depth.
@FunctionalInterface
public interface BundledLibraries {
    BundledLibraries NONE = name -> Optional.empty();

    Optional<Set<String>> declarations(String libraryName);
}
