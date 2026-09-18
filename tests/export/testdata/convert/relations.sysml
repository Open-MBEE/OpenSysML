package Relations {
    part def Base;
    part def Sub specializes Base {
        part b : Base;
    }
    part def Other specializes Sub {
        part r redefines b;
        part s subsets r;
    }
}
