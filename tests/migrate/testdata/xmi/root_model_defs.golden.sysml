package SEM {
    doc /* Signal electronics model. */
    part def 'Imaging Subsystem' {
        part BSED : SEM::BSED;
        attribute gain : Real;
        binding 'gain = BSED.gain' bind gain = BSED.gain;
    }
    part def BSED {
        attribute gain : Real;
    }
    constraint def 'Noise Figure' {
        in attribute nf : Real[1] {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "ConstraintParameter";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
            }
        }
        in attribute f : Real[1];
        /* not migrated: Constraint (_rule) {SysML} nf = 10 * log10(f) — opaque expression is not v2 expression syntax (language SysML) */
    }
    attribute def Real;
}
