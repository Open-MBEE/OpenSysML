package ReferenceSubsets {
    part def Vehicle;
    part vehicle_c1 : Vehicle;
    part def Person;
    part design {
        ref vehicle_design subsets vehicle_c1;
        ref spare subsets vehicle_c1;
        ref 'spare wheel' subsets vehicle_c1;
        ref fleet[0..*] subsets vehicle_c1;
        ref presidentOfCountry[0..1] : Person subsets vehicle_c1;
    }
}
