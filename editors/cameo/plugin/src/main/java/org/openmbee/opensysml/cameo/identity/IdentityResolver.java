package org.openmbee.opensysml.cameo.identity;

import java.util.List;
import org.openmbee.opensysml.cameo.model.ModelElement;

/** Maps a service symbol id back to the Cameo elements it may denote; ambiguity is preserved. */
@FunctionalInterface
public interface IdentityResolver {
  List<ModelElement> resolve(String symbolId);
}
