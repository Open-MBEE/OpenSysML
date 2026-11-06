package OccurrenceConfig {
    part def A { attribute prop_a : ScalarValues::Real; }
    part def B { attribute prop_b : ScalarValues::Real; }
    part system {
        part usage_a : A { attribute :>> prop_a = 100.0; }
        part usage_b : B { attribute :>> prop_b = usage_a.prop_a; }
        part usage_c : B { attribute :>> prop_b = usage_a.prop_a * 2.0; }
        part usage_d : B;
    }
}
