part def Plant {
    attribute runs : ScalarValues::Integer = 0;
    action fill { assign runs := runs + 1; }
}
part def BigPlant :> Plant { }
part def HugePlant :> BigPlant { }
part def RedefPlant :> Plant {
    action fill :>> fill { assign runs := runs + 10; }
}
part p : BigPlant;
part h : HugePlant;
part r : RedefPlant;
