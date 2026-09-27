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

    metadata def DeferredEvent {
        doc /* The annotated state deferred the signal that types signal in
             * the source: an occurrence while the state is active is kept and
             * dispatched once the state exits, unless a transition out of the
             * state accepts it. Written as `@DeferredEvent { ref :>> signal : Sig; }`. */
        ref signal : Base::Anything[1];
    }

    metadata def StandIn {
        doc /* The annotated element stands for no element of the source: the
             * migration made it up to write what the source left implicit,
             * such as the join several edges into one node wait through. No
             * diagram symbol stands at it. */
    }
}
