package org.openmbee.mdk.syson.publish;

import java.util.Map;
import java.util.concurrent.CompletableFuture;

import org.eclipse.sirius.components.annotations.spring.graphql.QueryDataFetcher;
import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.graphql.api.IDataFetcherWithFieldCoordinates;
import org.eclipse.sirius.components.graphql.api.IEditingContextDispatcher;
import org.eclipse.sirius.components.graphql.api.IExceptionWrapper;

import graphql.schema.DataFetchingEnvironment;
import tools.jackson.databind.ObjectMapper;

@QueryDataFetcher(type = "Mutation", field = "publishWithOpenSysML")
public class MutationPublishWithOpenSysMLDataFetcher
        implements IDataFetcherWithFieldCoordinates<CompletableFuture<IPayload>> {
    private final ObjectMapper objectMapper;
    private final IExceptionWrapper exceptionWrapper;
    private final IEditingContextDispatcher editingContextDispatcher;

    public MutationPublishWithOpenSysMLDataFetcher(ObjectMapper objectMapper, IExceptionWrapper exceptionWrapper,
            IEditingContextDispatcher editingContextDispatcher) {
        this.objectMapper = objectMapper;
        this.exceptionWrapper = exceptionWrapper;
        this.editingContextDispatcher = editingContextDispatcher;
    }

    @Override
    public CompletableFuture<IPayload> get(DataFetchingEnvironment environment) {
        Map<String, Object> argument = environment.getArgument("input");
        PublishWithOpenSysMLInput input = objectMapper.convertValue(argument, PublishWithOpenSysMLInput.class);
        return exceptionWrapper.wrapMono(
                () -> editingContextDispatcher.dispatchMutation(input.editingContextId(), input), input).toFuture();
    }
}
