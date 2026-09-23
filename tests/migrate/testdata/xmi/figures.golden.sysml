package Plant {
    item def Start;
    part def Pump {
        doc /* Moves the coolant. */
        state def Modes {
            view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {
                expose Modes;
                render Views::asInterconnectionDiagram;
            }
            entry; then Idle;
            state Idle;
            state Running;
            transition 'Idle accept Start then Running' first Idle accept Start then Running;
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
        }
    }
    part def Tank;
    view Blank {
        render Views::asTreeDiagram;
    }
    view Poster {
        render Views::asTreeDiagram;
    }
    view Unlisted {
        render Views::asTreeDiagram;
    }
    view Silent {
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
        }
        perform action 'figures Method' : 'Figures Method';
    }
    viewpoint 'Pictures Viewpoint' {
        action def 'Pictures Method' {
            first start then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'pictures Method' : 'Pictures Method';
    }
}
package 'Plant Documents' {
    part def 'Plant Handbook' {
        view behaviors :> Behaviors;
        view pictures :> Pictures;
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
        }
    }
}
