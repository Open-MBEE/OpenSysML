package Demo {
  requirement def MassLimit {
    attribute actual : ScalarValues::Real;
    require constraint { actual < 200.0 }
  }

  part def Vehicle {
    attribute mass : ScalarValues::Real = 100.0;
    part wheel : Wheel;
    constraint lightEnough { mass < 200.0 }
    constraint tiny { mass > 200.0 }
    requirement massLimit : MassLimit {
      attribute redefines actual = mass;
    }
  }

  part def Wheel;

  calc def twice { in x : ScalarValues::Integer; return : ScalarValues::Integer = x * 2; }

  state def Lifecycle {
    entry; then off;
    state off;
    state on;
    transition off_on first off accept Start then on;
  }
}
