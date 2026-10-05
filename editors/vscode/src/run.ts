// Runs the element a Run or Evaluate code lens names: the matching `sysml`
// check over the workspace's model files, as a task whose terminal shows the
// result.

import * as vscode from "vscode";

import { asRunElementArgs, runArguments, runPaths, runTitle } from "./runargs";

/** The languages whose documents `sysml` reads from disk, so a dirty one is saved first. */
const MODEL_LANGUAGES = new Set(["sysml", "kerml"]);

/** The command the server's lenses invoke; see internal/frontend/lsp/codelens.go. */
export const RUN_ELEMENT_COMMAND = "opensysml.runElement";

export class ElementRunner implements vscode.Disposable {
  private readonly command: vscode.Disposable;

  /** resolveTool finds the `sysml` executable, undefined when none is installed. */
  constructor(
    private readonly output: vscode.OutputChannel,
    private readonly resolveTool: () => string | undefined,
  ) {
    this.command = vscode.commands.registerCommand(RUN_ELEMENT_COMMAND, (argument?: unknown) => this.run(argument));
  }

  dispose(): void {
    this.command.dispose();
  }

  private async run(argument: unknown): Promise<void> {
    const args = asRunElementArgs(argument);
    if (!args) {
      void vscode.window.showErrorMessage("Run this from a Run or Evaluate code lens in a model file.");
      return;
    }
    const uri = vscode.Uri.parse(args.uri);
    if (uri.scheme !== "file") {
      void vscode.window.showErrorMessage(`${args.element} is not declared in a file on disk, so sysml cannot read it.`);
      return;
    }
    const tool = this.resolveTool();
    if (!tool) {
      void vscode.window.showWarningMessage(
        "Could not find sysml. Build it with `make build` beside sysml-lsp, or put it on your PATH.",
      );
      return;
    }
    const folders = (vscode.workspace.workspaceFolders ?? [])
      .filter((folder) => folder.uri.scheme === "file")
      .map((folder) => folder.uri.fsPath);
    const argv = runArguments(args, runPaths(uri.fsPath, folders));
    if (!argv) {
      void vscode.window.showErrorMessage(`No sysml check runs a ${args.kind}.`);
      return;
    }
    if (!(await this.saveModelDocuments())) {
      return;
    }
    this.output.appendLine(`Running ${tool} ${argv.join(" ")}`);
    const task = new vscode.Task(
      { type: "opensysml", element: args.element },
      vscode.workspace.getWorkspaceFolder(uri) ?? vscode.TaskScope.Workspace,
      runTitle(args),
      "SysML",
      new vscode.ProcessExecution(tool, argv),
    );
    task.presentationOptions = { reveal: vscode.TaskRevealKind.Always, clear: true, focus: false };
    await vscode.tasks.executeTask(task);
  }

  /**
   * Saves every dirty model document, hidden ones included, so the run reads what
   * the editor shows; false, after a message, when one could not be saved.
   */
  private async saveModelDocuments(): Promise<boolean> {
    const dirty = vscode.workspace.textDocuments.filter((d) => d.isDirty && MODEL_LANGUAGES.has(d.languageId));
    const saved = await Promise.all(dirty.map((d) => d.save()));
    const failed = dirty.filter((_, i) => !saved[i]);
    if (failed.length > 0) {
      void vscode.window.showErrorMessage(
        `Not run: ${failed.map((d) => vscode.workspace.asRelativePath(d.uri)).join(", ")} could not be saved.`,
      );
      return false;
    }
    return true;
  }
}
