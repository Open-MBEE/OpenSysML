package org.openmbee.mdk.magic.tools;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.LocalDate;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.time.temporal.ChronoUnit;
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
  private static final Pattern VERSION =
      Pattern.compile("^v?(\\d+)\\.(\\d+)\\.(\\d+)(?:-nightly-(\\d{8})-[0-9a-f]+)?$");
  /** Nights are counted from here; the count fills the pre-release digits below {@link #RELEASE}. */
  static final LocalDate NIGHTLY_EPOCH = LocalDate.of(2026, 1, 1);
  static final long RELEASE = 9_999;

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

  /** Packs the version into the integer the Resource Manager orders installs by: MMmmpp then four pre-release
   *  digits, 9999 for a release and the day since {@link #NIGHTLY_EPOCH} for a nightly (one day's nights tie). */
  static long internalVersion(String version) {
    Matcher m = VERSION.matcher(version);
    if (!m.matches()) {
      throw new IllegalArgumentException("version is not major.minor.patch[-nightly-yyyymmdd-commit]: " + version);
    }
    long major = Long.parseLong(m.group(1));
    long minor = Long.parseLong(m.group(2));
    long patch = Long.parseLong(m.group(3));
    if (minor > 99 || patch > 99) throw new IllegalArgumentException("minor and patch must be below 100: " + version);
    long preRelease = RELEASE;
    if (m.group(4) != null) {
      LocalDate built = LocalDate.parse(m.group(4), DateTimeFormatter.BASIC_ISO_DATE);
      preRelease = Math.min(Math.max(ChronoUnit.DAYS.between(NIGHTLY_EPOCH, built), 0), RELEASE - 1);
    }
    long internal = ((major * 100 + minor) * 100 + patch) * 10_000 + preRelease;
    if (internal > Integer.MAX_VALUE) throw new IllegalArgumentException("version exceeds the descriptor's integer: " + version);
    return internal;
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
