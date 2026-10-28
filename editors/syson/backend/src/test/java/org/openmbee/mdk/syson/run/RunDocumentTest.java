package org.openmbee.mdk.syson.run;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.DocumentForm;
import org.openmbee.opensysml.RenderedDocument;

class RunDocumentTest {

    @Test
    void takesTheFileNameTheServiceReports() {
        RunDocument document = RunDocument.of(RunDocumentForm.HTML,
                new RenderedDocument(DocumentForm.HTML, "<html></html>", "Reports-Mass.20Report~a1b2.html"),
                "Reports::'Mass Report'");

        assertThat(document.fileName()).isEqualTo("Reports-Mass.20Report~a1b2.html");
    }

    @Test
    void fallsBackToTheNameTheServicesLinksUseWhenItReportsNone() {
        assertThat(RunDocument.of(RunDocumentForm.MARKDOWN, new RenderedDocument(DocumentForm.MARKDOWN, "# Mass"),
                "MassReport").fileName()).isEqualTo("MassReport.md");
        assertThat(RunDocument.of(RunDocumentForm.HTML, new RenderedDocument(DocumentForm.HTML, ""),
                "Reports::'Mass Report'").fileName()).isEqualTo("Reports-Mass.20Report.html");
        assertThat(RunDocument.of(RunDocumentForm.HTML, new RenderedDocument(DocumentForm.HTML, ""),
                "Observatory::'Mass Appendix'").fileName()).isEqualTo("Observatory-Mass.20Appendix.html");
    }

    @Test
    void namesTheDocumentFileAsTheServiceDoes() {
        assertThat(RunDocument.fileName("Observatory::'Mass Appendix'", ".html"))
                .isEqualTo("Observatory-Mass.20Appendix.html");
        assertThat(RunDocument.fileName("Ünïcode::'a::b'", ".md")).isEqualTo("_.C3.9Cn.C3.AFcode-a-b.md");
        assertThat(RunDocument.fileName("'it\\'s'::'back\\\\slash'", ".md")).isEqualTo("it.5C.27s-back.5C.5Cslash.md");
        assertThat(RunDocument.fileName("CON", ".md")).isEqualTo("_.43ON.md");
        String cut = RunDocument.fileName("A".repeat(300), ".md");
        assertThat(cut).hasSize(255).endsWith(".md").contains("~");
        assertThat(cut.substring(0, cut.indexOf('~'))).isEqualTo("A".repeat(255 - 3 - 17));
    }
}
