package Unimported {
    attribute intIsInteger = 2 istype ScalarValues::Integer;
    attribute intHasInteger = 2 hastype ScalarValues::Integer;
    attribute intIsReal = 2 istype ScalarValues::Real;
    attribute intHasReal = 2 hastype ScalarValues::Real;
    attribute intIsNatural = 2 istype ScalarValues::Natural;
    attribute realIsReal = 2.5 istype ScalarValues::Real;
    attribute realHasReal = 2.5 hastype ScalarValues::Real;
    attribute realIsRational = 2.5 istype ScalarValues::Rational;
    attribute realHasRational = 2.5 hastype ScalarValues::Rational;
    attribute boolIsBoolean = true istype ScalarValues::Boolean;
    attribute boolHasBoolean = true hastype ScalarValues::Boolean;
    attribute stringIsString = "s" istype ScalarValues::String;
    attribute stringHasString = "s" hastype ScalarValues::String;
}

package Shadowed {
    attribute def Integer;
    attribute def Real;
    attribute def Boolean;
    attribute def String;

    attribute intIsOwnInteger = 2 istype Integer;
    attribute intHasOwnInteger = 2 hastype Integer;
    attribute intIsInteger = 2 istype ScalarValues::Integer;
    attribute intHasInteger = 2 hastype ScalarValues::Integer;
    attribute realIsOwnReal = 2.5 istype Real;
    attribute realHasOwnReal = 2.5 hastype Real;
    attribute realIsReal = 2.5 istype ScalarValues::Real;
    attribute boolIsOwnBoolean = true istype Boolean;
    attribute boolIsBoolean = true istype ScalarValues::Boolean;
    attribute stringIsOwnString = "s" istype String;
    attribute stringIsString = "s" istype ScalarValues::String;
}
