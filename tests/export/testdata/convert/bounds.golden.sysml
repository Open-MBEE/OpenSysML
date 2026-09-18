package Bounds {
    part def Vehicle;
    part def Fleet {
        part exact : Vehicle[3];
        part range : Vehicle[1..4];
        part many : Vehicle[0..*];
        part loose : Vehicle[*] ordered nonunique;
        ref part shared : Vehicle;
        composite part owned : Vehicle;
        derived attribute count : Integer;
    }
}
