package Plant {
    item def Start;
    part def Pump {
        doc /* Moves the coolant. */
        state def Modes {
            view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {
                expose Modes;
                @DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }
                metadata DiagramLayout::Layout about Plant::Pump::Modes::Idle { x = 40; y = 100; width = 120; height = 50; }
                metadata DiagramLayout::Layout about Plant::Pump::Modes::Running { x = 260; y = 100; width = 120; height = 50; }
                metadata DiagramLayout::Route about 'Idle accept Start then Running' { points = (160, 125, 210, 125, 210, 130, 260, 130); }
                metadata DiagramLayout::Style about Plant::Pump::Modes::Idle { fill = "#E1E1C3"; line = "#99995C"; text = "#000000"; font = "Arial"; fontSize = 11; bold = true; }
                metadata DiagramLayout::Note about Plant::Pump::Modes::Idle { text = "Moves the coolant."; x = 40; y = 200; width = 150; height = 40; }
                @DiagramLayout::Note { text = "Draft only"; x = 600; y = 20; width = 80; height = 12; }
                render Views::asInterconnectionDiagram;
            }
            entry; then Idle;
            state Idle;
            state Running;
            transition 'Idle accept Start then Running' first Idle accept Start then Running;
            metadata MigrationMetadata::SynthesizedName about 'Idle accept Start then Running';
        }
        action def Prime {
            view Priming : StandardViewDefinitions::ActionFlowView {
                expose Prime;
                render Views::asInterconnectionDiagram;
            }
            first start then Fill;
            action Fill {
                /* body not migrated (the body is empty) */
            }
            first Fill then Vent;
            action Vent {
                /* body not migrated (the body is empty) */
            }
            first Vent then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
    }
    part def Tank;
    view Blank {
        @DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }
        render Views::asTreeDiagram;
    }
    view Poster {
        @DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }
        @DiagramLayout::Note { text = "The plant from the north"; x = 40; y = 260; width = 300; height = 40; }
        render Views::asTreeDiagram;
    }
    view Unlisted {
        expose Tank;
        expose Pump;
        @DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }
        metadata DiagramLayout::Layout about Tank { x = 100; y = 100; width = 120; height = 60; }
        render Views::asTreeDiagram;
    }
    view Silent {
        render Views::asTreeDiagram;
    }
    view Stale {
        expose Tank;
        @DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }
        metadata DiagramLayout::Layout about Tank { x = 100; y = 100; width = 120; height = 60; }
        render Views::asTreeDiagram;
    }
    view Partial {
        expose Tank;
        expose Pump;
        @DiagramLayout::Canvas { unit = "px"; width = 810; height = 610; }
        metadata DiagramLayout::Layout about Tank { x = 100; y = 100; width = 120; height = 60; }
        metadata DiagramLayout::Layout about Pump { x = 300; y = 100; width = 120; height = 60; }
        render Views::asTreeDiagram;
    }
}
package 'Plant Viewpoints' {
    viewpoint 'Figures Viewpoint' {
        action def 'Figures Method' {
            first start then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'figures Method' : 'Figures Method';
    }
    viewpoint 'Pictures Viewpoint' {
        action def 'Pictures Method' {
            first start then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'pictures Method' : 'Pictures Method';
    }
    viewpoint 'Shown Viewpoint' {
        action def 'Shown Method' {
            first start then 'Collect Things On Diagram';
            action 'Collect Things On Diagram';
            first 'Collect Things On Diagram' then 'Shown Blocks';
            action 'Shown Blocks' {
                first start then Name;
                action Name;
            }
            first 'Shown Blocks' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'shown Method' : 'Shown Method';
    }
}
package 'Plant Documents' {
    part def 'Plant Handbook' {
        view behaviors :> Behaviors;
        view pictures :> Pictures;
        view shown :> Shown;
        /* applied stereotype «Document» */
    }
    view Behaviors {
        satisfy 'Plant Viewpoints'::'Figures Viewpoint';
        expose Plant::Pump::Modes::'Pump Modes';
        expose Plant::Pump::Prime::Priming;
    }
    view Pictures {
        satisfy 'Plant Viewpoints'::'Pictures Viewpoint';
        expose Plant::Blank;
        expose Plant::Poster;
        expose Plant::Unlisted;
        expose Plant::Silent;
        expose Plant::Stale;
    }
    view Shown {
        satisfy 'Plant Viewpoints'::'Shown Viewpoint';
        expose Plant::Partial;
        expose Plant::Unlisted;
    }
    calc def 'Plant Handbook Shown Blocks Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Named(qualifiedName = ("Plant::Tank", "Plant::Pump")),
            properties = ("name"))
    }
    part def 'Plant Handbook Document' :> DocumentQueries::Document {
        attribute redefines title = "Plant Handbook";
        ref modes : Plant::Pump::Modes;
        ref prime : Plant::Pump::Prime;
        part Behaviors : DocumentQueries::Section {
            attribute redefines title = "Behaviors";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Pump Modes";
                ref redefines source = modes.'Pump Modes';
            }
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "How the pump behaves";
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Priming";
                ref redefines source = prime.Priming;
            }
            part 'paragraph 2' : DocumentQueries::Paragraph {
                attribute redefines text = "How the pump is primed";
            }
        }
        part Pictures : DocumentQueries::Section {
            attribute redefines title = "Pictures";
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "Nothing to see";
            }
            part 'paragraph 2' : DocumentQueries::Paragraph {
                attribute redefines text = "The plant, photographed";
            }
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Unlisted";
                ref redefines source = Plant::Unlisted;
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Stale";
                ref redefines source = Plant::Stale;
            }
        }
        part Shown : DocumentQueries::Section {
            attribute redefines title = "Shown";
            part table : DocumentQueries::Table {
                attribute redefines caption = "Shown Blocks";
                calc rows : 'Plant Handbook Shown Blocks Rows';
            }
        }
    }
}
