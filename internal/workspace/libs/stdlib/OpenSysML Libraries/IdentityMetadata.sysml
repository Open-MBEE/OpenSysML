standard library package IdentityMetadata {
    doc /* Binds notation to repository identity. Non-normative OpenSysML
         * extension, proposed for standardization; see the design record. */

    metadata def ElementId {
        doc /* The annotated element is the repository element with this id. */
        attribute id : ScalarValues::String;
    }

    metadata def ProjectRef {
        doc /* Elements below the annotated namespace that carry ElementId
             * resolve against this project and branch. */
        attribute projectId : ScalarValues::String;
        attribute branch : ScalarValues::String[0..1];
        attribute org : ScalarValues::String[0..1];
    }
}
