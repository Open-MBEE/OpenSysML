package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.cameo.identity.ElementTree;
import org.openmbee.opensysml.cameo.identity.IdentityIndex;
import org.openmbee.opensysml.cameo.model.ModelElement;
import org.openmbee.opensysml.cameo.model.SimpleElement;

class IdentityIndexTest {
  @Test
  void preservesAmbiguousCandidates() {
    var first = new SimpleElement("1", "Demo::Thing", "Thing", "a");
    var second = new SimpleElement("2", "Demo::Thing", "Thing", "b");
    assertEquals(List.of(first, second), IdentityIndex.of(List.of(first, second)).resolve("Demo::Thing"));
  }

  @Test
  void resolvesV1NamesWithQuotesAndIds() {
    var mass = new SimpleElement("_18_0_mass", "Requirements::Mass Requirement", "Mass Requirement", "m");
    var index = IdentityIndex.of(List.of(mass));
    assertEquals(List.of(mass), index.resolve("Requirements::'Mass Requirement'"));
    assertEquals(List.of(mass), index.resolve("_18_0_mass"));
    assertTrue(index.resolve("Requirements::Other").isEmpty());
  }

  @Test
  void resolvesV2InstancePathsToTheirDeclaration() {
    var part = new SimpleElement("p", "Demo::Vehicle::wheel", "wheel", "w");
    var index = IdentityIndex.of(List.of(part));
    assertEquals(List.of(part), index.resolve("Demo::Vehicle::wheel"));
    assertEquals(List.of(part), index.resolve("Demo::Vehicle::wheel.mass"));
  }

  @Test
  void treeWalkIndexesEveryAdaptableNode() {
    Map<String, List<String>> children = Map.of("root", List.of("A", "B"), "A", List.of("A::x"), "B", List.of(), "A::x", List.of());
    IdentityIndex index = ElementTree.index("root", children::get,
        node -> node.equals("root") ? Optional.<ModelElement>empty() : Optional.of(new SimpleElement(node, node, node, node)));
    assertEquals(3, index.size());
    assertEquals("A::x", index.resolve("A::x").get(0).id());
  }
}
