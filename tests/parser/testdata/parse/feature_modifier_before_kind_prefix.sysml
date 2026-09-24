package P {
    datatype D {
        derived var feature a : D[1];
        private derived var feature b : D[1];
        derived composite var feature c : D[0..*];
    }
}
