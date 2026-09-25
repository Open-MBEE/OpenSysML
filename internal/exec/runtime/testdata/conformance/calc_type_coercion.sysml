package test {
    // Test type coercion: Integer + Real → Real
    calc testCoercion {
        in intVal : Integer;
        in realVal : Real;
        intVal + realVal  // Integer coerced to Real
    }
}
