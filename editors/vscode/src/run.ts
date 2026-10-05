// Runs the element a Run or Evaluate code lens names: the matching `sysml`
// check on the element's file, as a task whose terminal shows the result.

import * as vscode from "vscode";

import { asRunElementArgs, runArguments, runTitle } from "./runargs";

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
    const argv = runArguments(args, uri.fsPath);
    if (!argv) {
      void vscode.window.showErrorMessage(`No sysml check runs a ${args.kind}.`);
      return;
    }
    const editor = vscode.window.visibleTextEditors.find((e) => e.document.uri.toString() === uri.toString());
    if (editor?.document.isDirty) {
      await editor.document.save();
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
}
