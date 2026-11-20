package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.Optional;
import java.util.prefs.Preferences;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.repository.PublishSettings;

/** What the plugin keeps about a model's publication between sessions. */
class PublishSettingsTest {
  @Test
  void remembersWhereAModelWasPublishedButNeverAToken() throws Exception {
    Preferences root = Preferences.userRoot().node("org/openmbee/mdk/magic/test-" + System.nanoTime());
    try {
      PublishSettings settings = new PublishSettings(root, "/models/Mission.mdzip");
      assertEquals("", settings.url());
      assertTrue(settings.lastSeen("http://localhost:8083", "p1", "main").isEmpty());

      settings.remember("http://localhost:8083", "p1", "main");
      settings.sawHead("http://localhost:8083", "p1", "main", "commit-1");

      PublishSettings reopened = new PublishSettings(root, "/models/Mission.mdzip");
      assertEquals("http://localhost:8083", reopened.url());
      assertEquals("p1", reopened.project());
      assertEquals("main", reopened.branch());
      assertEquals(Optional.of("commit-1"), reopened.lastSeen("http://localhost:8083", "p1", "main"));
      assertTrue(reopened.lastSeen("http://localhost:8083", "p1", "other").isEmpty());
      assertTrue(reopened.lastSeen("http://localhost:8083", "", "").isEmpty());
      assertEquals("", new PublishSettings(root, "/models/Other.mdzip").project());
    } finally {
      root.removeNode();
    }
  }
}
