package org.openmbee.opensysml.syson.run;

import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

import org.springframework.stereotype.Service;

@Service
public class RunResultStore {
    private final ConcurrentHashMap<String, RunResult> results = new ConcurrentHashMap<>();

    public void put(String editingContextId, RunResult result) {
        results.put(editingContextId, result);
    }

    public Optional<RunResult> latest(String editingContextId) {
        return Optional.ofNullable(results.get(editingContextId));
    }

    public void clear(String editingContextId) {
        results.remove(editingContextId);
    }
}
