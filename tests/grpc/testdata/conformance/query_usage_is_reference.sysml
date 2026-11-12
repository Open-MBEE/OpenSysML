package Fleet {
    part def Engine;
    part def Person;
    part def Car {
        variation part engine : Engine {
            variant part four;
        }
        part spare : Engine;
        ref part driver : Person;
    }
}
