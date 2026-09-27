standard library package MigrationMetadata {
    doc /* What a migration from another modeling language had to make up
         * because its source left it unstated. Non-normative OpenSysML
         * extension, proposed for standardization; see the design record. */

    metadata def SynthesizedName {
        doc /* The annotated element was anonymous in the source; the
             * migration named it so the notation could declare and refer to
             * it. The name keys the element but is not one a picture shows. */
    }

    metadata def LibraryNameAvoided {
        doc /* The annotated top-level package was named, in the source model,
             * like a standard library package, which a qualified name starting
             * with it would resolve to; the migration renamed it and recorded
             * the source name here and in its report. */
        attribute sourceName : ScalarValues::String;
    }
}
