package ReferenceSubsets {
    part def Vehicle;
    part vehicle_c1 : Vehicle;
    part def Person;
    part design {
        ref vehicle_design :> vehicle_c1;
        ref spare subsets vehicle_c1;
        ref 'spare wheel' subsets vehicle_c1;
        ref fleet[0..*] :> vehicle_c1;
        ref presidentOfCountry[0..1] : Person :> vehicle_c1;
    }
}
