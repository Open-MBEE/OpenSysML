package org.eclipse.sirius.components.annotations.spring.graphql;

import static java.lang.annotation.ElementType.TYPE;
import static java.lang.annotation.RetentionPolicy.RUNTIME;

import java.lang.annotation.Documented;
import java.lang.annotation.Retention;
import java.lang.annotation.Target;

import org.springframework.stereotype.Component;

@Documented
@Retention(RUNTIME)
@Target(TYPE)
@Component
public @interface QueryDataFetcher {
    String type();

    String field();
}
