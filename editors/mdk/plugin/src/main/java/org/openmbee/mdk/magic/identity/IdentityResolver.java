package org.openmbee.mdk.magic.identity;

import java.util.List;
import org.openmbee.mdk.magic.model.ModelElement;

/** Maps a service symbol id back to the Cameo elements it may denote; ambiguity is preserved. */
@FunctionalInterface
public interface IdentityResolver {
  List<ModelElement> resolve(String symbolId);
}
