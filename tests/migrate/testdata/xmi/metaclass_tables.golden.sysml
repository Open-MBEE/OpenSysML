package Model {
    package Plant {
        package Structure {
            part def Pump {
                doc /* Moves fluid through the plant. */
                attribute mass : ScalarValues::Real;
                part valve : Valve;
                abstract action prime;
                action def halt {
                    first start then final;
                    action final terminate;
                    metadata MigrationMetadata::SynthesizedName about final;
                }
                action 'halt 2' : halt;
                state def Cycle {
                    in ref context : Pump[1];
                    entry; then Idle;
                    state Idle;
                    state Running {
                        entry action : halt { in ref :>> context = Cycle::context; }
                    }
                    transition first Idle then Running;
                }
            }
            part def Valve;
            enum def Mode {
                on;
                off;
            }
            part p1 : Pump {
                attribute :>> mass = 12.5;
            }
            dependency needs from Pump to Valve;
        }
        package Views {
            view Overview;
            view def Operations {
                viewpoint operations {
                    subject;
                }
                satisfy operations;
                metadata MigrationMetadata::SynthesizedName about operations;
            }
        }
    }
    package Tables {
        view 'Packageable Elements' {
            expose Plant::Structure::Pump;
            expose 'Packageable Elements Document';
            render Views::asElementTable;
        }
        calc def 'Packageable Elements Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("Model::Plant")));
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::Union(
                        source = DocumentQueries::WhereType(source = candidates, type = ("Package", "Definition", "ViewUsage", "ViewpointUsage", "Dependency", "SatisfyRequirementUsage", "AllocationUsage")),
                        other = DocumentQueries::Except(
                            source = DocumentQueries::Except(
                                source = DocumentQueries::WhereType(source = candidates, type = ("Usage")),
                                exclude = DocumentQueries::Union(
                                    source = DocumentQueries::WhereType(source = candidates, type = ("ConnectionUsage")),
                                    other = DocumentQueries::WhereType(source = candidates, type = ("MetadataUsage")))),
                            exclude = DocumentQueries::Descendants(
                                source = DocumentQueries::WhereType(
                                    source = DocumentQueries::Union(
                                        source = DocumentQueries::Named(qualifiedName = ("Model::Plant")),
                                        other = DocumentQueries::Descendants(
                                            source = DocumentQueries::Union(
                                                source = DocumentQueries::WhereType(
                                                    source = DocumentQueries::Named(qualifiedName = ("Model::Plant")),
                                                    type = ("Package")),
                                                other = DocumentQueries::WhereType(source = candidates, type = ("Package"))),
                                            maxDepth = 1)),
                                    type = ("Type"))))),
                    property = "qualifiedName",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("qualifiedName"))
        }
        part def 'Packageable Elements Document' :> DocumentQueries::Document {
            attribute redefines title = "Packageable Elements";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Packageable Elements";
                calc rows : 'Packageable Elements Rows';
            }
        }
        view Namespaces {
            expose Plant::Structure::Pump;
            expose 'Namespaces Document';
            render Views::asElementTable;
        }
        calc def 'Namespaces Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("Model::Plant")));
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::Union(
                        source = DocumentQueries::WhereType(source = candidates, type = ("Package", "Definition", "ViewUsage", "ViewpointUsage", "StateUsage")),
                        other = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Model::Plant::Structure::Pump::prime")),
                            exclude = DocumentQueries::Except(
                                source = DocumentQueries::Named(qualifiedName = ("Model::Plant::Structure::Pump::prime")),
                                exclude = candidates))),
                    property = "qualifiedName",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("qualifiedName"))
        }
        part def 'Namespaces Document' :> DocumentQueries::Document {
            attribute redefines title = "Namespaces";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Namespaces";
                calc rows : 'Namespaces Rows';
            }
        }
        view Types {
            expose Plant::Structure::Pump;
            expose 'Types Document';
            render Views::asElementTable;
        }
        calc def 'Types Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Model::Plant"))),
                        type = ("Definition", "ViewUsage", "ViewpointUsage")),
                    property = "qualifiedName",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("qualifiedName"))
        }
        part def 'Types Document' :> DocumentQueries::Document {
            attribute redefines title = "Types";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Types";
                calc rows : 'Types Rows';
            }
        }
        view Classifiers {
            expose Plant::Structure::Pump;
            expose 'Classifiers Document';
            render Views::asElementTable;
        }
        calc def 'Classifiers Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Model::Plant"))),
                        type = ("Definition", "ViewUsage", "ViewpointUsage")),
                    property = "qualifiedName",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("qualifiedName"))
        }
        part def 'Classifiers Document' :> DocumentQueries::Document {
            attribute redefines title = "Classifiers";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Classifiers";
                calc rows : 'Classifiers Rows';
            }
        }
        view Operations {
            expose Plant::Structure::Pump;
            expose 'Operations Document';
            render Views::asElementTable;
        }
        calc def 'Operations Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("Model::Plant")));
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::Union(
                        source = DocumentQueries::Except(
                            source = DocumentQueries::WhereType(source = candidates, type = ("ActionDefinition")),
                            exclude = DocumentQueries::WhereType(source = candidates, type = ("StateDefinition", "CalculationDefinition"))),
                        other = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Model::Plant::Structure::Pump::prime")),
                            exclude = DocumentQueries::Except(
                                source = DocumentQueries::Named(qualifiedName = ("Model::Plant::Structure::Pump::prime")),
                                exclude = candidates))),
                    property = "qualifiedName",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("qualifiedName"))
        }
        part def 'Operations Document' :> DocumentQueries::Document {
            attribute redefines title = "Operations";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Operations";
                calc rows : 'Operations Rows';
            }
        }
        view Diagrams {
            expose Plant::Structure::Pump;
            expose 'Diagrams Document';
            render Views::asElementTable;
        }
        calc def 'Diagrams Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Union(
                            source = DocumentQueries::Named(qualifiedName = ("Model")),
                            other = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Model")))),
                        type = ("ViewUsage")),
                    property = "qualifiedName",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("qualifiedName"))
        }
        part def 'Diagrams Document' :> DocumentQueries::Document {
            attribute redefines title = "Diagrams";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Diagrams";
                calc rows : 'Diagrams Rows';
            }
        }
    }
    view 'Model Overview' {
        expose Plant;
        expose Tables;
        render Views::asTreeDiagram;
    }
}
