package org.openmbee.opensysml.syson.export;

import java.util.ArrayDeque;
import java.util.Deque;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.Symbol;

// A library package is bundled when an empty model already resolves it: the service's libraries
// are the only names an empty model has. The empty model is parsed afresh for each new name, so
// the service evicting it from its cache costs nothing; answers are kept for the connection's lifetime.
public final class ServiceLibraries implements BundledLibraries {
    private final Connection connection;
    private final Map<String, Optional<Set<String>>> known = new ConcurrentHashMap<>();

    public ServiceLibraries(Connection connection) {
        this.connection = connection;
    }

    @Override
    public Optional<Set<String>> declarations(String libraryName) {
        if (libraryName == null || libraryName.isBlank()) return Optional.empty();
        return known.computeIfAbsent(libraryName, name -> {
            Model empty = connection.parse("");
            return empty.find(name).map(root -> declarationsUnder(empty, root, name + "::"));
        });
    }

    // Every id under the root, relative to it, walking child ids depth first; members a scope
    // only imports or inherits lie outside the prefix and are left out.
    private static Set<String> declarationsUnder(Model model, Symbol root, String prefix) {
        Set<String> names = new LinkedHashSet<>();
        Deque<Symbol> pending = new ArrayDeque<>();
        pending.push(root);
        while (!pending.isEmpty()) {
            for (String id : pending.pop().childIds()) {
                if (!id.startsWith(prefix) || !names.add(id.substring(prefix.length()))) continue;
                model.findSymbol(id).ifPresent(pending::push);
            }
        }
        return names;
    }
}
