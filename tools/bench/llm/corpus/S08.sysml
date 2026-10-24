package Office {
    part def Desk;
    part def Chair;
    part def Monitor;
    part def Computer;

    part def Workstation {
        part desk : Desk;
        part chair : Chair;
        part monitors : Monitor[1..2];
        part computer : Computer;
    }

    part def Office {
        part workstations : Workstation[1..*];
        part printer : Printer[0..1];
    }

    part def Printer;
}
