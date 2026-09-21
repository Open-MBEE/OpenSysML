package org.eclipse.sirius.components.graphql.api;

import java.util.List;

import org.eclipse.sirius.components.annotations.spring.graphql.QueryDataFetcher;

import graphql.schema.DataFetcher;
import graphql.schema.FieldCoordinates;

public interface IDataFetcherWithFieldCoordinates<T> extends DataFetcher<T> {
    default List<FieldCoordinates> getFieldCoordinates() {
        QueryDataFetcher annotation = this.getClass().getAnnotation(QueryDataFetcher.class);
        FieldCoordinates coordinates = annotation == null ? null
                : FieldCoordinates.coordinates(annotation.type(), annotation.field());
        return List.of(coordinates);
    }
}
