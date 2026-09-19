package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class JsonTest {

  @Test
  void readsAConnectErrorBody() {
    Object body = Json.parse("{\"code\":\"not_found\",\"message\":\"model not found\"}");
    assertEquals("not_found", Json.stringMember(body, "code"));
    assertEquals("model not found", Json.stringMember(body, "message"));
  }

  @Test
  void readsNestedStructure() {
    Object body = Json.parse("{\"a\":[1,2.5,true,null,\"x\"],\"b\":{\"c\":{}}}");
    assertTrue(body instanceof Map<?, ?>);
    List<?> a = (List<?>) ((Map<?, ?>) body).get("a");
    assertEquals(5, a.size());
    assertEquals(1.0, a.get(0));
    assertEquals(2.5, a.get(1));
    assertEquals(Boolean.TRUE, a.get(2));
    assertNull(a.get(3));
    assertEquals("x", a.get(4));
  }

  @Test
  void readsEscapes() {
    assertEquals("a\"b\\c\nd\u00e9", Json.parse("\"a\\\"b\\\\c\\nd\\u00e9\""));
  }

  @Test
  void absentMembersAreEmptyRatherThanNull() {
    assertEquals("", Json.stringMember(Json.parse("{}"), "code"));
    assertEquals("", Json.stringMember(Json.parse("[]"), "code"));
    assertEquals("", Json.stringMember(Json.parse("{\"code\":7}"), "code"));
  }

  @Test
  void malformedInputIsRejected() {
    assertThrows(IllegalArgumentException.class, () -> Json.parse("{"));
    assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\" 1}"));
    assertThrows(IllegalArgumentException.class, () -> Json.parse("[1,]x"));
    assertThrows(IllegalArgumentException.class, () -> Json.parse("nope"));
    assertThrows(IllegalArgumentException.class, () -> Json.parse("{} {}"));
  }
}
