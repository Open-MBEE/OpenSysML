package org.openmbee.opensysml.syson.run;

import org.openmbee.opensysml.DocumentForm;

public enum RunDocumentForm {
    MARKDOWN(DocumentForm.MARKDOWN, "md"),
    HTML(DocumentForm.HTML, "html");

    private final DocumentForm form;
    private final String extension;

    RunDocumentForm(DocumentForm form, String extension) {
        this.form = form;
        this.extension = extension;
    }

    public DocumentForm form() {
        return form;
    }

    public String extension() {
        return extension;
    }
}
