// Exporting a rendering to a file: which forms the server writes, how each is
// offered and saved, and the pick → request → save sequence, kept apart from
// VS Code so the sequence is testable with a fake host.
import { RENDER_FORMS_CAPABILITY, RenderParams, RenderResult } from "./protocol";

/** The forms `opensysml/render` writes, as the wire contract documents them, for a server that lists none itself. */
export const DOCUMENTED_FORMS: readonly string[] = ["text", "mermaid", "markdown", "dot", "plantuml"];

/** How an exported form is saved: the file extension and the save dialog's filter name. */
export interface ExportFile {
  extension: string;
  filter: string;
}

interface FormDescription extends ExportFile {
  description: string;
}

// The file a form is saved as and how the picker describes it, by the form's name.
const FORMS: Record<string, FormDescription> = {
  mermaid: { extension: ".mmd", filter: "Mermaid", description: "Mermaid diagram, the model's positions as layout comments" },
  dot: { extension: ".dot", filter: "Graphviz DOT", description: "Graphviz DOT graph; a positioned view names its layout engine" },
  plantuml: { extension: ".puml", filter: "PlantUML", description: "PlantUML diagram in the Pilot visualizer's style" },
  markdown: { extension: ".md", filter: "Markdown", description: "Markdown pipe table, for a table view" },
  text: { extension: ".txt", filter: "Text", description: "Text form, one line per element" },
};

/** Whatever form the server wrote that the extension does not know is saved as text. */
const UNKNOWN_FORM: ExportFile = { extension: ".txt", filter: "Text" };

/**
 * serverForms lists the forms a server writes: the ones it advertises under
 * `openSysmlRenderForms` in its experimental capabilities, else the documented five.
 */
export function serverForms(experimental: Record<string, unknown> | undefined): string[] {
  const advertised = experimental?.[RENDER_FORMS_CAPABILITY];
  if (Array.isArray(advertised) && advertised.length > 0 && advertised.every((form) => typeof form === "string" && form !== "")) {
    return [...new Set(advertised as string[])];
  }
  return [...DOCUMENTED_FORMS];
}

/** exportFile is how a form the server wrote is saved. */
export function exportFile(form: string): ExportFile {
  const known = FORMS[form];
  return known ? { extension: known.extension, filter: known.filter } : UNKNOWN_FORM;
}

/** exportFileName is the default name an export of a document is saved under: its stem with the form's extension. */
export function exportFileName(documentName: string, form: string): string {
  return documentName.replace(/\.(sysml|kerml)$/, "") + exportFile(form).extension;
}

/** One entry of the form picker. */
export interface FormPickItem {
  label: string;
  description: string;
  value: string;
}

/** formPickItems is the picker over the server's forms, each labeled by the file it saves as. */
export function formPickItems(forms: readonly string[]): FormPickItem[] {
  return forms.map((form) => {
    const known = FORMS[form];
    return {
      label: form,
      description: known ? `${known.extension} — ${known.description}` : `${UNKNOWN_FORM.extension} — saved as text`,
      value: form,
    };
  });
}

/** What the export sequence needs from the editor, so the sequence itself has no VS Code in it. */
export interface ExportHost {
  /** pickForm offers the forms; undefined when the user dismisses the pick. */
  pickForm(items: FormPickItem[], title: string): Promise<string | undefined>;
  /** render sends `opensysml/render` and yields the rendering, or throws the server's error. */
  render(params: RenderParams): Promise<RenderResult>;
  /** pickSaveLocation is the save dialog; undefined when the user cancels it. */
  pickSaveLocation(defaultName: string, file: ExportFile): Promise<string | undefined>;
  /** write saves the artifact where the user chose. */
  write(location: string, artifact: string): Promise<void>;
}

/** What an export of a document's view came to; a failure says which step failed. */
export type ExportOutcome =
  | { kind: "saved"; form: string; location: string }
  | { kind: "cancelled" }
  | { kind: "failed"; step: "render" | "save"; message: string };

export interface ExportRequest {
  /** The document rendered, as the server names it. */
  uri: string;
  /** The document's name, which the saved file is named after. */
  documentName: string;
  /** The view rendered; empty for the document's own. */
  view: string;
  /** The forms offered, in the order the server lists them. */
  forms: readonly string[];
}

/**
 * exportRendering asks which form to save, renders the view in it and writes the
 * artifact where the user chooses; the file is named and filtered by the form the
 * server answered with, which is the one asked for.
 */
export async function exportRendering(host: ExportHost, request: ExportRequest): Promise<ExportOutcome> {
  const form = await host.pickForm(formPickItems(request.forms), `Export ${request.documentName} as which form?`);
  if (form === undefined) {
    return { kind: "cancelled" };
  }
  let result: RenderResult;
  try {
    result = await host.render({ textDocument: { uri: request.uri }, view: request.view, form });
  } catch (err) {
    return { kind: "failed", step: "render", message: errorMessage(err) };
  }
  let location: string | undefined;
  try {
    location = await host.pickSaveLocation(exportFileName(request.documentName, result.form), exportFile(result.form));
    if (location === undefined) {
      return { kind: "cancelled" };
    }
    await host.write(location, result.artifact);
  } catch (err) {
    return { kind: "failed", step: "save", message: errorMessage(err) };
  }
  return { kind: "saved", form: result.form, location };
}

function errorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    return String((err as { message: unknown }).message);
  }
  return String(err);
}
