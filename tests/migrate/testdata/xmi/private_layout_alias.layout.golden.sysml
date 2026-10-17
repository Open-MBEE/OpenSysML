package Model {
    part def A {
        private part x : B;
        part b : B;
        view 'A BDD' {
            expose A::x;
            expose B::x;
            @DiagramLayout::Canvas { unit = "px"; width = 190; height = 0; }
            metadata DiagramLayout::Layout about 'x (A)' { :>> DiagramLayout::Layout::x = 10; y = -20; width = 80; height = -40; }
            metadata DiagramLayout::Layout about B::x { :>> DiagramLayout::Layout::x = 110; y = -20; width = 80; height = -40; }
            render Views::asTreeDiagram;
        }
        private alias 'x (A)' for x;
        metadata MigrationMetadata::SynthesizedName about 'x (A)';
    }
    part def B {
        attribute x;
    }
}
