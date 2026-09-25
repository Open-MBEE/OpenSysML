package org.openmbee.opensysml.syson.run;

import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

import org.eclipse.sirius.components.collaborative.api.IEditingContextEventProcessor;
import org.eclipse.sirius.components.collaborative.api.IEditingContextEventProcessorRegistry;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import reactor.core.Disposable;

@Service
public class RunResultStore {
    private final ConcurrentHashMap<String, RunResult> results = new ConcurrentHashMap<>();
    private final ConcurrentHashMap<String, Disposable> subscriptions = new ConcurrentHashMap<>();
    private final ObjectProvider<IEditingContextEventProcessorRegistry> registry;

    @Autowired
    public RunResultStore(ObjectProvider<IEditingContextEventProcessorRegistry> registry) {
        this.registry = registry;
    }

    public RunResultStore() {
        this.registry = null;
    }

    public void put(String editingContextId, RunResult result) {
        results.put(editingContextId, result);
        subscriptions.computeIfAbsent(editingContextId, this::subscribeToDisposal);
    }

    public Optional<RunResult> latest(String editingContextId) {
        return Optional.ofNullable(results.get(editingContextId));
    }

    public void clear(String editingContextId) {
        results.remove(editingContextId);
        Disposable subscription = subscriptions.remove(editingContextId);
        if (subscription != null) subscription.dispose();
    }

    private Disposable subscribeToDisposal(String editingContextId) {
        if (registry == null) {
            return null;
        }
        IEditingContextEventProcessorRegistry processorRegistry = registry.getIfAvailable();
        if (processorRegistry == null) {
            return null;
        }
        IEditingContextEventProcessor processor = processorRegistry.getEditingContextEventProcessors().stream()
                .filter(candidate -> editingContextId.equals(candidate.getEditingContextId()))
                .findFirst().orElse(null);
        if (processor == null) {
            // Unit tests can exercise the store without a live Sirius event processor.
            return null;
        }
        return processor.getOutputEvents()
                .doFinally(signal -> clear(editingContextId))
                .subscribe(payload -> {
                }, error -> clear(editingContextId));
    }
}
