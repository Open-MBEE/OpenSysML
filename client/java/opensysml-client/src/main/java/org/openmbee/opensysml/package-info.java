/**
 * The OpenSysML client: {@link org.openmbee.opensysml.Connection} opens a service, {@link org.openmbee.opensysml.Model}
 * reads a model it parsed.
 *
 * <p>This package is the whole public surface of v1. It covers parsing a file or inline notation,
 * evaluating an expression, looking a symbol up, instantiating a part, and capability negotiation.
 * The edit API, RDF conversion, verification, queries and generated model-ergonomics types are
 * deliberately not here rather than half-wrapped; the {@code org.openmbee.opensysml.proto} messages the
 * service defines for them are generated and on the classpath for a caller who needs them now.
 *
 * <p>Nothing here exposes a generated protobuf type or builder. Values are immutable, absence is
 * {@link java.util.Optional}, and every failure is an unchecked
 * {@link org.openmbee.opensysml.OpenSysMLException}.
 */
package org.openmbee.opensysml;
