package P {
    package DocumentQueries {}
    calc def Q :> $::DocumentQueries::Query {
        $::DocumentQueries::Named(qualifiedName = "P")
    }
    calc def R :> $::DocumentQueries::Query {
        return result = $::DocumentQueries::Named(qualifiedName = "P");
    }
    attribute x = $::P::y + 1;
}
