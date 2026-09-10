package IndividualDefinitions {
    part def Astronaut;
    individual part def 'Neil Armstrong' specializes Astronaut;
    item def Sample;
    individual item def 'Sample 10005';
    individual occurrence def FirstLanding;
    individual action def PlantFlag;
    abstract individual part def Spacecraft;
    individual def Eagle specializes Spacecraft;
    part def Mission {
        part crew : Astronaut[*];
        individual part commander : 'Neil Armstrong' subsets crew;
        individual item returned : 'Sample 10005';
        individual occurrence landing : FirstLanding;
        individual action flag : PlantFlag;
        individual part lander : Eagle;
    }
}
