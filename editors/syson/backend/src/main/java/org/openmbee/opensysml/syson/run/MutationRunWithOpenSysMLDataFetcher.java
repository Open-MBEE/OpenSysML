package org.openmbee.opensysml.syson.run;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;

import org.eclipse.sirius.components.annotations.spring.graphql.QueryDataFetcher;
import org.eclipse.sirius.components.core.api.ErrorPayload;
import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.graphql.api.IDataFetcherWithFieldCoordinates;
import org.eclipse.sirius.components.graphql.api.IEditingContextDispatcher;
import org.eclipse.sirius.components.graphql.api.IExceptionWrapper;

import graphql.schema.DataFetchingEnvironment;
import reactor.core.publisher.Mono;
import tools.jackson.databind.ObjectMapper;

@QueryDataFetcher(type = "Mutation", field = "runWithOpenSysML")
public class MutationRunWithOpenSysMLDataFetcher implements IDataFetcherWithFieldCoordinates<CompletableFuture<IPayload>> {
    private final ObjectMapper objectMapper;
    private final IExceptionWrapper exceptionWrapper;
    private final IEditingContextDispatcher editingContextDispatcher;

    public MutationRunWithOpenSysMLDataFetcher(ObjectMapper objectMapper, IExceptionWrapper exceptionWrapper,
            IEditingContextDispatcher editingContextDispatcher) {
        this.objectMapper = objectMapper;
        this.exceptionWrapper = exceptionWrapper;
        this.editingContextDispatcher = editingContextDispatcher;
    }

    @Override
    @SuppressWarnings("unchecked")
    public CompletableFuture<IPayload> get(DataFetchingEnvironment environment) {
        Map<String, Object> argument = environment.getArgument("input");
        // GraphQL represents named inputs as a list; the input record uses a map.
        Map<String, String> values = new LinkedHashMap<>();
        String inputError = collectInputs(
                (List<Map<String, String>>) argument.getOrDefault("inputs", List.of()), values);
        Map<String, Object> convertedArgument = new LinkedHashMap<>(argument);
        convertedArgument.put("inputs", values);
        RunWithOpenSysMLInput converted = objectMapper.convertValue(convertedArgument, RunWithOpenSysMLInput.class);
        RunWithOpenSysMLInput input = new RunWithOpenSysMLInput(converted.id(), converted.editingContextId(),
                converted.objectId(), converted.operation(), values, converted.events(), converted.arguments(),
                converted.schedule(), converted.subject());
        if (inputError != null) {
            String message = inputError;
            return exceptionWrapper.wrapMono(() -> Mono.just(new ErrorPayload(input.id(), message)),
                    input).toFuture();
        }
        return exceptionWrapper.wrapMono(() -> editingContextDispatcher.dispatchMutation(input.editingContextId(), input),
                input).toFuture();
    }

    private static String collectInputs(List<Map<String, String>> inputs, Map<String, String> values) {
        for (Map<String, String> value : inputs) {
            String name = value.get("name");
            if (name == null || name.isBlank()) {
                return "input name must not be blank";
            }
            if (values.containsKey(name)) {
                return "duplicate input name: " + name;
            }
            values.put(name, value.get("expression"));
        }
        return null;
    }
}