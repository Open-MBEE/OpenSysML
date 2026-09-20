package org.openmbee.opensysml.syson.run;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.stream.Collectors;

import org.eclipse.sirius.components.annotations.spring.graphql.QueryDataFetcher;
import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.graphql.api.IDataFetcherWithFieldCoordinates;
import org.eclipse.sirius.components.graphql.api.IEditingContextDispatcher;
import org.eclipse.sirius.components.graphql.api.IExceptionWrapper;

import graphql.schema.DataFetchingEnvironment;
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
        Map<String, String> values = ((List<Map<String, String>>) argument.getOrDefault("inputs", List.of())).stream()
                .collect(Collectors.toMap(value -> value.get("name"), value -> value.get("expression")));
        Map<String, Object> convertedArgument = new LinkedHashMap<>(argument);
        convertedArgument.put("inputs", values);
        RunWithOpenSysMLInput converted = objectMapper.convertValue(convertedArgument, RunWithOpenSysMLInput.class);
        RunWithOpenSysMLInput input = new RunWithOpenSysMLInput(converted.id(), converted.editingContextId(),
                converted.objectId(), converted.operation(), values, converted.events(), converted.arguments(),
                converted.schedule(), converted.subject());
        return exceptionWrapper.wrapMono(() -> editingContextDispatcher.dispatchMutation(input.editingContextId(), input),
                input).toFuture();
    }
}
