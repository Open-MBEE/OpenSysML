package Inclusion {
    use case def Step;
    use case shared : Step;
    use case def Whole {
        include shared;
    }
}
