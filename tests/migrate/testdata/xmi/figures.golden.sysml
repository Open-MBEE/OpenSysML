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
            metadata MigrationMetadata::SynthesizedName about 'Idle accept Start then Running';
        }
        action def Prime {
            view Priming : StandardViewDefinitions::ActionFlowView {
                expose Prime;
                render Views::asInterconnectionDiagram;
            }
            first start then Fill;
            action Fill;
            first Fill then Vent;
            action Vent;
            first Vent then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
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
    view Stale {
        expose Pump;
        render Views::asTreeDiagram;
    }
    view Roster {
        expose Tank;
        render Views::asElementTable;
    }
    view Partial {
        expose Tank;
        render Views::asTreeDiagram;
    }
}
package 'Plant Viewpoints' {
    view def 'Figures Viewpoint' {
        viewpoint 'figures Viewpoint' {
            subject;
        }
        satisfy 'figures Viewpoint';
        action def 'Figures Method' {
            first start then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'figures Method' : 'Figures Method';
        metadata MigrationMetadata::SynthesizedName about 'figures Viewpoint';
    }
    view def 'Pictures Viewpoint' {
        viewpoint 'pictures Viewpoint' {
            subject;
        }
        satisfy 'pictures Viewpoint';
        action def 'Pictures Method' {
            first start then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'pictures Method' : 'Pictures Method';
        metadata MigrationMetadata::SynthesizedName about 'pictures Viewpoint';
    }
    view def 'Shown Viewpoint' {
        viewpoint 'shown Viewpoint' {
            subject;
        }
        satisfy 'shown Viewpoint';
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
        metadata MigrationMetadata::SynthesizedName about 'shown Viewpoint';
    }
}
package 'Plant Documents' {
    occurrence def 'Plant Handbook' {
        view behaviors :> Behaviors;
        view pictures :> Pictures;
        view shown :> Shown;
        /* applied stereotype «Document» */
    }
    view Behaviors : 'Plant Viewpoints'::'Figures Viewpoint' {
        expose Plant::Pump::Modes::'Pump Modes';
        expose Plant::Pump::Prime::Priming;
    }
    view Pictures : 'Plant Viewpoints'::'Pictures Viewpoint' {
        expose Plant::Blank;
        expose Plant::Poster;
        expose Plant::Unlisted;
        expose Plant::Silent;
        expose Plant::Stale;
        expose Plant::Roster;
    }
    view Shown : 'Plant Viewpoints'::'Shown Viewpoint' {
        expose Plant::Partial;
        expose Plant::Unlisted;
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
                attribute redefines caption = "Stale";
                ref redefines source = Plant::Stale;
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Roster";
                ref redefines source = Plant::Roster;
            }
        }
        part Shown : DocumentQueries::Section {
            attribute redefines title = "Shown";
            /* not migrated: «TableStructure» StructuredActivityNode 'Shown Blocks' — the elements it shows pass through «CollectThingsOnDiagram» Plant Viewpoints::Shown Viewpoint::Shown Method::Collect Things On Diagram is not migrated: it collects what the SysML Block Definition Diagram 'Partial' shows beyond the 1 element the tool lists, whose symbols cannot be read and what the SysML Block Definition Diagram 'Unlisted' shows, which the archive does not record */
        }
    }
}
