package org.openmbee.opensysml.cameo.tools;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.stream.Stream;

/** Writes plugin.xml with a runtime library entry for the plugin jar and every jar under lib/. */
public final class PluginDescriptorWriter {
  private PluginDescriptorWriter() {}

  public static void main(String[] args) throws IOException {
    if (args.length != 4) {
      throw new IllegalArgumentException("usage: PluginDescriptorWriter <version> <pluginJar> <libDir> <out>");
    }
    Files.writeString(Path.of(args[3]), render(args[0], args[1], libraries(Path.of(args[2]))), StandardCharsets.UTF_8);
  }

  static List<String> libraries(Path libDir) throws IOException {
    try (Stream<Path> files = Files.list(libDir)) {
      return files.map(path -> path.getFileName().toString()).filter(name -> name.endsWith(".jar")).sorted().toList();
    }
  }

  static String render(String version, String pluginJar, List<String> libraries) {
    StringBuilder xml = new StringBuilder("""
        <?xml version="1.0" encoding="UTF-8"?>
        <plugin id="org.openmbee.opensysml.cameo"
                name="OpenSysML"
                version="%s"
                provider-name="Open-MBEE"
                ownClassloader="true"
                class-lookup="LocalFirst">
          <class>org.openmbee.opensysml.cameo.OpenSysMLPlugin</class>
          <runtime>
        """.formatted(version));
    xml.append("    <library name=\"").append(pluginJar).append("\"/>\n");
    for (String library : libraries) xml.append("    <library name=\"lib/").append(library).append("\"/>\n");
    return xml.append("  </runtime>\n</plugin>\n").toString();
  }
}
