// A second document of the self-model, written in AsciiDoc: the markup the
// document pipeline parses outside the process. It lives beside the eight
// model files rather than among them so that `make self-model`, which renders
// every document the files declare, needs no converter; the README names the
// command that renders this one.
package OpenSysMLAsciiDoc {
    private import DocumentQueries::*;
    private import ScalarValues::*;

    // The query root: the table below tabulates the parsers of this pipeline.
    part documentPipeline : OpenSysMLSurfaces::DocumentPipeline;

    part def MarkupNote :> Document {
        attribute redefines title = "Prose in Two Markups";

        rep language "asciidoc" /* The architecture document is written in Markdown; this note
         * is written in _AsciiDoc_, the other of the markups a prose body may
         * be written in -- sysml:value[documentPipeline.plans.markupLanguages]
         * -- and it is rendered by the same planner, so <<parsers>> describes
         * one planner fed by two parsers and <<converter>> what AsciiDoc needs
         * that Markdown does not.
         */

        part parsers : Section {
            attribute redefines title = "One planner, two parsers";

            rep language "asciidoc" /* A body's language picks its parser, the parser hands the
             * planner blocks, and the blocks are the content kinds every document
             * is made of: sysml:value[documentPipeline.plans.contentKinds]. Inside
             * a paragraph the runs are sysml:value[documentPipeline.plans.runKinds],
             * and a plan, once compiled, is immutable
             * (sysml:value[documentPipeline.plans.immutablePlans, immutablePlans:]). <<languages>>
             * is a query over sysml:ref[documentPipeline.plans, the planner]'s
             * siblings, not a table typed here.
             */

            part languages : Table {
                attribute redefines caption = "The two parsers, and which one leaves the process";
                calc rows : OpenSysMLDocument::MarkupLanguages {
                    in redefines root = documentPipeline;
                }
            }
        }

        rep language "asciidoc" /* [[converter]]
         * == The converter
         *
         * An AsciiDoc body needs an external tool
         * (sysml:value[documentPipeline.asciidocProse.needsExternalTool, needsExternalTool:]):
         * Asciidoctor.js, run through sysml:value[documentPipeline.asciidocProse.converterScript]
         * and found by the variable sysml:value[documentPipeline.asciidocProse.toolEnv] or as
         * `asciidoctor-json` on the path. Without it the document fails to plan with a typed
         * error naming both -- a body is *never* rendered as literal text -- which is why this
         * note is a document of its own.
         *
         * * `./scripts/download-doc-pdf-toolchain.sh` installs the pinned converter.
         * * The variable it prints points the renderer at it.
         * * Both parsers live in sysml:value[documentPipeline.asciidocProse.goPackage].
         *
         * NOTE: The Markdown parser (sysml:value[documentPipeline.markdownProse.markupLanguage])
         * runs in the process and needs nothing installed.
         */
    }
}
