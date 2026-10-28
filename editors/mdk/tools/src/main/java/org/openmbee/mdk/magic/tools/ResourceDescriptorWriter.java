package org.openmbee.mdk.magic.tools;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.LocalDate;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;

/**
 * Writes the Resource Manager descriptor ({@code data/resourcemanager/MDR_Plugin_Magic_MDK_91120_descriptor.xml})
 * in the vendor's {@code resourceDescriptor} form, listing every file the distribution zip installs.
 */
public final class ResourceDescriptorWriter {
  public static final String RESOURCE_ID = "91120";
  public static final String FILE_NAME = "MDR_Plugin_Magic_MDK_" + RESOURCE_ID + "_descriptor.xml";
  /** Oldest release the plugin is built for; see docs/internals/design/mdk-plugin.md §1.1. */
  public static final String MINIMUM_RELEASE = "2024x";
  static final String PLUGIN_DIR = "plugins/org.openmbee.mdk.magic";
  static final String BRIDGE_DIR = "plugins/org.openmbee.mdk/extensions";
  private static final Pattern VERSION = Pattern.compile("^v?(\\d+)\\.(\\d+)\\.(\\d+)");

  private ResourceDescriptorWriter() {}

  public static void main(String[] args) throws IOException {
    if (args.length != 6) {
      throw new IllegalArgumentException(
          "usage: ResourceDescriptorWriter <version> <pluginJar> <bridgeJar> <libDir> <binDir> <out>");
    }
    List<String> files = installedFiles(args[1], args[2], Path.of(args[3]), Path.of(args[4]));
    Files.writeString(Path.of(args[5]), render(args[0], LocalDate.now(ZoneOffset.UTC).toString(), files), StandardCharsets.UTF_8);
  }

  static List<String> installedFiles(String pluginJar, String bridgeJar, Path libDir, Path binDir) throws IOException {
    List<String> files = new ArrayList<>();
    files.add(PLUGIN_DIR + "/plugin.xml");
    files.add(PLUGIN_DIR + "/" + pluginJar);
    for (String jar : PluginDescriptorWriter.libraries(libDir)) files.add(PLUGIN_DIR + "/lib/" + jar);
    for (String binary : names(binDir)) files.add(PLUGIN_DIR + "/bin/" + binary);
    files.add(BRIDGE_DIR + "/" + bridgeJar);
    files.add("data/resourcemanager/" + FILE_NAME);
    return files;
  }

  private static List<String> names(Path dir) throws IOException {
    if (!Files.isDirectory(dir)) return List.of();
    try (Stream<Path> files = Files.list(dir)) {
      return files.filter(Files::isRegularFile).map(path -> path.getFileName().toString()).sorted().toList();
    }
  }

  /** {@code major.minor.patch} packed into one integer the Resource Manager can order installs by. */
  static long internalVersion(String version) {
    Matcher m = VERSION.matcher(version);
    if (!m.find()) throw new IllegalArgumentException("version is not major.minor.patch: " + version);
    return Long.parseLong(m.group(1)) * 1_000_000L + Long.parseLong(m.group(2)) * 1_000L + Long.parseLong(m.group(3));
  }

  static String render(String version, String date, List<String> files) {
    long internal = internalVersion(version);
    StringBuilder xml = new StringBuilder("""
        <?xml version="1.0" encoding="UTF-8"?>
        <resourceDescriptor critical="false"
                            date="%s"
                            description="Runs, instantiates, verifies and analyzes SysML models with OpenSysML and exports them as SysML v2; for Cameo Systems Modeler and MagicDraw %s or later"
                            homePage="https://github.com/Open-MBEE/OpenSysML"
                            id="%s"
                            mdVersionMax="higher"
                            mdVersionMin="19.0"
                            name="Magic MDK"
                            product="Magic MDK"
                            type="Plugin">
          <version human="%s" internal="%d" resource="%d"/>
          <provider name="Open-MBEE" homePage="https://www.openmbee.org"/>
        """.formatted(date, MINIMUM_RELEASE, RESOURCE_ID, version, internal, internal));
    for (String edition : List.of("Reader", "Community", "Standard", "Professional Java", "Professional C++",
        "Professional C#", "Professional ArcStyler", "Professional EFFS ArcStyler", "OptimalJ", "Professional",
        "Architect", "Enterprise")) {
      xml.append("  <edition>").append(edition).append("</edition>\n");
    }
    xml.append("  <installation>\n");
    for (String file : files) {
      xml.append("    <file from=\"").append(file).append("\" to=\"").append(file).append("\"/>\n");
    }
    return xml.append("  </installation>\n</resourceDescriptor>\n").toString();
  }
}
