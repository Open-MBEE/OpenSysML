package fUML_Library {
    package PrimitiveBehaviors {
        package ListFunctions {
            action def ListSize {
                in list : ScalarValues::String[0..*];
                out result : ScalarValues::Integer;
                /* body not migrated (the body is empty) */
            }
        }
    }
}
part def Recorder {
    action def Tally {
        first start then size;
        action size : fUML_Library::PrimitiveBehaviors::ListFunctions::ListSize;
        first size then get;
        action get {
            in list : ScalarValues::String[0..*] ordered nonunique = "n=3";
            in index : ScalarValues::Integer = 1;
            out result : ScalarValues::String;
            /* not migrated: CallBehaviorAction 'get' — the behavior fUML_Library::PrimitiveBehaviors::ListFunctions::ListGet it calls has no v2 declaration */
        }
        first get then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
