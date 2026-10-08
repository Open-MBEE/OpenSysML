package 'Safety Profile' {
    metadata def Critical;
    metadata def Vital :> Critical;
}
package 'Cost Profile' {
    metadata def Critical;
}
package Plant {
    part def Pump {
        satisfy requirement : 'Flow Rate';
        view 'Pump Layout' {
            expose Pump;
            render Views::asTreeDiagram;
        }
        @'Safety Profile'::Critical;
    }
    part def Valve {
        view 'Valve Layout' {
            expose Valve;
            render Views::asTreeDiagram;
        }
        @'Cost Profile'::Critical;
    }
    part def Tank {
        view 'Tank Layout' {
            expose Tank;
            render Views::asTreeDiagram;
        }
        @'Safety Profile'::Vital;
    }
    requirement def <'D-1'> 'Flow Rate' {
        doc /* The pump delivers the rated flow. */
    }
    view 'Plant Layout' {
        expose Pump;
        expose Tank;
        render Views::asTreeDiagram;
    }
}
package Spares {
    part def Hose {
        /* applied stereotype «Critical» */
    }
    part def Gasket;
}
package 'Depot Viewpoints' {
    view def 'Critical Items Viewpoint' {
        viewpoint 'critical Items Viewpoint' {
            subject;
        }
        satisfy 'critical Items Viewpoint';
        action def 'Critical Items Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Critical Items';
            action 'Critical Items' {
                first start then Name;
                action Name;
            }
            first 'Critical Items' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'critical Items Method' : 'Critical Items Method';
        metadata MigrationMetadata::SynthesizedName about 'critical Items Viewpoint';
    }
    view def 'Exact Critical Layouts Viewpoint' {
        viewpoint 'exact Critical Layouts Viewpoint' {
            subject;
        }
        satisfy 'exact Critical Layouts Viewpoint';
        action def 'Exact Critical Layouts Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Collect Owned Elements 2';
            action 'Collect Owned Elements 2';
            first 'Collect Owned Elements 2' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'exact Critical Layouts Method' : 'Exact Critical Layouts Method';
        metadata MigrationMetadata::SynthesizedName about 'exact Critical Layouts Viewpoint';
    }
    view def 'Critical Figures Viewpoint' {
        viewpoint 'critical Figures Viewpoint' {
            subject;
        }
        satisfy 'critical Figures Viewpoint';
        action def 'Critical Figures Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'critical Figures Method' : 'Critical Figures Method';
        metadata MigrationMetadata::SynthesizedName about 'critical Figures Viewpoint';
    }
    view def 'Uncritical Figures Viewpoint' {
        viewpoint 'uncritical Figures Viewpoint' {
            subject;
        }
        satisfy 'uncritical Figures Viewpoint';
        action def 'Uncritical Figures Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'uncritical Figures Method' : 'Uncritical Figures Method';
        metadata MigrationMetadata::SynthesizedName about 'uncritical Figures Viewpoint';
    }
    view def 'Critical Layouts Viewpoint' {
        viewpoint 'critical Layouts Viewpoint' {
            subject;
        }
        satisfy 'critical Layouts Viewpoint';
        action def 'Critical Layouts Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Collect Owned Elements 2';
            action 'Collect Owned Elements 2';
            first 'Collect Owned Elements 2' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'critical Layouts Method' : 'Critical Layouts Method';
        metadata MigrationMetadata::SynthesizedName about 'critical Layouts Viewpoint';
    }
    view def 'Satisfier Figures Viewpoint' {
        viewpoint 'satisfier Figures Viewpoint' {
            subject;
        }
        satisfy 'satisfier Figures Viewpoint';
        action def 'Satisfier Figures Method' {
            first start then 'Collect Satisfiers';
            action 'Collect Satisfiers';
            first 'Collect Satisfiers' then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'satisfier Figures Method' : 'Satisfier Figures Method';
        metadata MigrationMetadata::SynthesizedName about 'satisfier Figures Viewpoint';
    }
    view def 'Satisfier Layouts Viewpoint' {
        viewpoint 'satisfier Layouts Viewpoint' {
            subject;
        }
        satisfy 'satisfier Layouts Viewpoint';
        action def 'Satisfier Layouts Method' {
            first start then 'Collect Satisfiers';
            action 'Collect Satisfiers';
            first 'Collect Satisfiers' then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'satisfier Layouts Method' : 'Satisfier Layouts Method';
        metadata MigrationMetadata::SynthesizedName about 'satisfier Layouts Viewpoint';
    }
    view def 'Owned Figures Viewpoint' {
        viewpoint 'owned Figures Viewpoint' {
            subject;
        }
        satisfy 'owned Figures Viewpoint';
        action def 'Owned Figures Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Collect Owned Elements 2';
            action 'Collect Owned Elements 2';
            first 'Collect Owned Elements 2' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'owned Figures Method' : 'Owned Figures Method';
        metadata MigrationMetadata::SynthesizedName about 'owned Figures Viewpoint';
    }
}
package 'Depot Handbook' {
    occurrence def 'Depot Handbook' {
        view criticalItems :> 'Critical Items';
        view exactCriticalLayouts :> 'Exact Critical Layouts';
        view criticalFigures :> 'Critical Figures';
        view uncriticalFigures :> 'Uncritical Figures';
        view criticalLayouts :> 'Critical Layouts';
        view satisfierFigures :> 'Satisfier Figures';
        view satisfierLayouts :> 'Satisfier Layouts';
        view ownedFigures :> 'Owned Figures';
        /* applied stereotype «Document» */
    }
    view 'Critical Items' : 'Depot Viewpoints'::'Critical Items Viewpoint' {
        expose Plant::**;
        expose Spares::**;
    }
    view 'Exact Critical Layouts' : 'Depot Viewpoints'::'Exact Critical Layouts Viewpoint' {
        expose Plant::**;
    }
    view 'Critical Figures' : 'Depot Viewpoints'::'Critical Figures Viewpoint' {
        expose Spares::**;
    }
    view 'Uncritical Figures' : 'Depot Viewpoints'::'Uncritical Figures Viewpoint' {
        expose Plant::**;
        expose Spares::**;
    }
    view 'Critical Layouts' : 'Depot Viewpoints'::'Critical Layouts Viewpoint' {
        expose Plant::**;
    }
    view 'Satisfier Figures' : 'Depot Viewpoints'::'Satisfier Figures Viewpoint' {
        expose Plant::'Flow Rate';
    }
    view 'Satisfier Layouts' : 'Depot Viewpoints'::'Satisfier Layouts Viewpoint' {
        expose Plant::'Flow Rate';
    }
    view 'Owned Figures' : 'Depot Viewpoints'::'Owned Figures Viewpoint' {
        expose Spares::**;
    }
    calc def 'Depot Handbook Critical Items Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereMetadata(
                source = DocumentQueries::Descendants(
                    source = DocumentQueries::Named(qualifiedName = ("Plant", "Spares"))),
                'metadata' = ("Safety Profile::Critical")),
            properties = ("name"))
    }
    part def 'Depot Handbook Document' :> DocumentQueries::Document {
        attribute redefines title = "Depot Handbook";
        ref pump : Plant::Pump;
        ref valve : Plant::Valve;
        ref tank : Plant::Tank;
        part 'Critical Items' : DocumentQueries::Section {
            attribute redefines title = "Critical Items";
            part table : DocumentQueries::Table {
                attribute redefines caption = "Critical Items";
                calc rows : 'Depot Handbook Critical Items Rows';
            }
        }
        part 'Exact Critical Layouts' : DocumentQueries::Section {
            attribute redefines title = "Exact Critical Layouts";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Pump Layout";
                ref redefines source = pump.'Pump Layout';
            }
        }
        part 'Critical Figures' : DocumentQueries::Section {
            attribute redefines title = "Critical Figures";
        }
        part 'Uncritical Figures' : DocumentQueries::Section {
            attribute redefines title = "Uncritical Figures";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Plant Layout";
                ref redefines source = Plant::'Plant Layout';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Pump Layout";
                ref redefines source = pump.'Pump Layout';
            }
            part 'diagram 3' : DocumentQueries::Diagram {
                attribute redefines caption = "Valve Layout";
                ref redefines source = valve.'Valve Layout';
            }
            part 'diagram 4' : DocumentQueries::Diagram {
                attribute redefines caption = "Tank Layout";
                ref redefines source = tank.'Tank Layout';
            }
        }
        part 'Critical Layouts' : DocumentQueries::Section {
            attribute redefines title = "Critical Layouts";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Pump Layout";
                ref redefines source = pump.'Pump Layout';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Tank Layout";
                ref redefines source = tank.'Tank Layout';
            }
        }
        part 'Satisfier Figures' : DocumentQueries::Section {
            attribute redefines title = "Satisfier Figures";
        }
        part 'Satisfier Layouts' : DocumentQueries::Section {
            attribute redefines title = "Satisfier Layouts";
            /* not migrated: «Image» CallBehaviorAction 'Image' — the diagrams it shows are not known: «CollectByDirectedRelationshipStereotypes» Depot Viewpoints::Satisfier Layouts Viewpoint::Satisfier Layouts Method::Collect Satisfiers follows relationships to elements only the query finds */
        }
        part 'Owned Figures' : DocumentQueries::Section {
            attribute redefines title = "Owned Figures";
            /* not migrated: «Image» CallBehaviorAction 'Image' — the diagrams it shows are not known: «FilterByStereotypes» Depot Viewpoints::Owned Figures Viewpoint::Owned Figures Method::Filter By Stereotypes keeps or drops the «Block» Class Spares::Hose, which cannot be told before the query runs */
        }
    }
}
