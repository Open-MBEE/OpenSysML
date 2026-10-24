package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * The external tool call an analysis would make, previewed without running the tool: what {@code
 * sysml -tool-dry-run} prints, structured.
 *
 * @param action the qualified name of the tool-executing action reached
 * @param tool the tool the action names
 * @param version the tool version the manifest declares, empty when it declares none
 * @param manifest the manifest file the tool was found in
 * @param executable the program that would run
 * @param protocol how the tool is spoken to
 * @param args the command line the tool would get
 * @param env environment entries set for it
 * @param cwd the working directory it would run in
 * @param stdin how standard input is fed
 * @param stdinData the text written to standard input
 * @param inputFile the input file written for the tool, when it reads one
 * @param outputDir whether an output directory would be made
 * @param inputs each value handed over
 * @param outputs each result read back
 * @param replySource where the tool's answer would be read from
 * @param lines the preview as {@code sysml -tool-dry-run} prints it
 */
public record ToolDryRun(
    String action,
    String tool,
    String version,
    String manifest,
    String executable,
    String protocol,
    List<String> args,
    List<String> env,
    String cwd,
    String stdin,
    String stdinData,
    Optional<InputFile> inputFile,
    boolean outputDir,
    List<Input> inputs,
    List<Output> outputs,
    String replySource,
    List<String> lines) {

  /**
   * Validates and copies the preview.
   *
   * @param action the action reached
   * @param tool the tool named
   * @param version the tool version
   * @param manifest the manifest file
   * @param executable the program
   * @param protocol the protocol
   * @param args the command line
   * @param env the environment entries
   * @param cwd the working directory
   * @param stdin how standard input is fed
   * @param stdinData the standard input text
   * @param inputFile the input file, when one is written
   * @param outputDir whether an output directory is made
   * @param inputs the values handed over
   * @param outputs the results read back
   * @param replySource where the answer is read from
   * @param lines the printed preview
   */
  public ToolDryRun {
    Objects.requireNonNull(action, "action");
    Objects.requireNonNull(tool, "tool");
    Objects.requireNonNull(version, "version");
    Objects.requireNonNull(manifest, "manifest");
    Objects.requireNonNull(executable, "executable");
    Objects.requireNonNull(protocol, "protocol");
    args = List.copyOf(args);
    env = List.copyOf(env);
    Objects.requireNonNull(cwd, "cwd");
    Objects.requireNonNull(stdin, "stdin");
    Objects.requireNonNull(stdinData, "stdinData");
    Objects.requireNonNull(inputFile, "inputFile");
    inputs = List.copyOf(inputs);
    outputs = List.copyOf(outputs);
    Objects.requireNonNull(replySource, "replySource");
    lines = List.copyOf(lines);
  }

  /**
   * The input file the tool would read.
   *
   * @param name the file name
   * @param format the format it is written in
   * @param data its content
   */
  public record InputFile(String name, String format, String data) {
    /**
     * Validates the file.
     *
     * @param name the file name
     * @param format the format
     * @param data the content
     */
    public InputFile {
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(format, "format");
      Objects.requireNonNull(data, "data");
    }
  }

  /**
   * A value handed to the tool.
   *
   * @param variable the model variable it comes from
   * @param parameter the tool parameter it is bound to
   * @param value the value, as text
   * @param unit the unit it is in, empty when unitless
   */
  public record Input(String variable, String parameter, String value, String unit) {
    /**
     * Validates the input.
     *
     * @param variable the variable
     * @param parameter the parameter
     * @param value the value
     * @param unit the unit
     */
    public Input {
      Objects.requireNonNull(variable, "variable");
      Objects.requireNonNull(parameter, "parameter");
      Objects.requireNonNull(value, "value");
      Objects.requireNonNull(unit, "unit");
    }
  }

  /**
   * A result read back from the tool.
   *
   * @param variable the model variable it is written to
   * @param parameter the tool parameter it is read from
   */
  public record Output(String variable, String parameter) {
    /**
     * Validates the output.
     *
     * @param variable the variable
     * @param parameter the parameter
     */
    public Output {
      Objects.requireNonNull(variable, "variable");
      Objects.requireNonNull(parameter, "parameter");
    }
  }
}
