package Observatory {
    package 'Interface Types' {
        attribute def Bits;
        port def USB {
            inout attribute data : Bits[1];
        }
        port def Serial {
            in attribute cmd : Bits[1];
        }
        port def Spare;
    }
    package Assemblies {
        part def Collimator {
            port usb : Observatory::'Interface Types'::USB {
                doc /* Camera link. */
            }
            port ctl : Observatory::'Interface Types'::Serial;
        }
        part def Mirror {
            port tilt : Observatory::'Interface Types'::Serial {
                doc /* Tilt command. */
            }
        }
        part def Hub {
            port usb : Observatory::'Interface Types'::USB;
            port serial : Observatory::'Interface Types'::Serial;
        }
        part def 'Optical Bench' {
            part collimator : Collimator;
            part mirror : Mirror;
        }
        part def 'Controller Rack' {
            part hub : Hub;
        }
        part def 'Observatory System' {
            part bench : 'Optical Bench';
            part rack : 'Controller Rack';
            connection 'camera link' connect bench.collimator.usb to rack.hub.usb {
                doc /* Camera data to the rack. */
            }
            connection 'tilt link' connect bench.mirror.tilt to rack.hub.serial;
        }
    }
    package Viewpoints {
        viewpoint 'Totals Viewpoint' {
            doc /* Count the interfaces of each type. */
            action def 'Totals Method' {
                first start then 'Collect Owned Elements';
                action 'Collect Owned Elements';
                first 'Collect Owned Elements' then 'Filter By Metaclasses';
                action 'Filter By Metaclasses';
                first 'Filter By Metaclasses' then 'Sort By Name';
                action 'Sort By Name';
                first 'Sort By Name' then 'Interface Totals';
                action 'Interface Totals' {
                    first start then 'Interface Type';
                    action 'Interface Type';
                    first 'Interface Type' then 'Number of Interfaces';
                    action 'Number of Interfaces';
                    first 'Number of Interfaces' then Iterated;
                    action Iterated;
                    first Iterated then done;
                }
                first 'Interface Totals' then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about final;
            }
            perform action 'totals Method' : 'Totals Method';
        }
        viewpoint 'Connections Viewpoint' {
            doc /* List the connections. */
            action def 'Connections Method' {
                first start then 'Collect Owned Elements';
                action 'Collect Owned Elements';
                first 'Collect Owned Elements' then 'Filter By Metaclasses';
                action 'Filter By Metaclasses';
                first 'Filter By Metaclasses' then 'Table of Connections';
                action 'Table of Connections' {
                    first start then 'Point of Interface on Optical Bench';
                    action 'Point of Interface on Optical Bench';
                    first 'Point of Interface on Optical Bench' then 'Point of Interface on Controller Rack';
                    action 'Point of Interface on Controller Rack';
                    first 'Point of Interface on Controller Rack' then Type;
                    action Type;
                    first Type then Direction;
                    action Direction;
                    first Direction then Description;
                    action Description;
                    first Description then done;
                }
                first 'Table of Connections' then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about final;
            }
            perform action 'connections Method' : 'Connections Method';
        }
        viewpoint 'Interfaces Viewpoint' {
            doc /* Table the interfaces of each assembly. */
            action def 'Interfaces Method' {
                first start then 'Collect Owned Elements';
                action 'Collect Owned Elements';
                first 'Collect Owned Elements' then 'Filter By Metaclasses';
                action 'Filter By Metaclasses';
                first 'Filter By Metaclasses' then 'Sort By Name';
                action 'Sort By Name';
                first 'Sort By Name' then 'Per Assembly';
                action 'Per Assembly' {
                    first start then Assembly;
                    action Assembly;
                    first Assembly then 'Collect Types';
                    action 'Collect Types';
                    first 'Collect Types' then 'Interfaces Table';
                    action 'Interfaces Table' {
                        first start then Name;
                        action Name;
                        first Name then Description;
                        action Description {
                            first start then 'Collect Owned Elements';
                            action 'Collect Owned Elements';
                            first 'Collect Owned Elements' then 'Filter By Metaclasses';
                            action 'Filter By Metaclasses';
                            first 'Filter By Metaclasses' then done;
                        }
                        first Description then Interfaces;
                        action Interfaces {
                            first start then 'Collect Owned Elements';
                            action 'Collect Owned Elements';
                            first 'Collect Owned Elements' then 'Filter By Metaclasses';
                            action 'Filter By Metaclasses';
                            first 'Filter By Metaclasses' then 'Collect Types';
                            action 'Collect Types';
                            first 'Collect Types' then done;
                        }
                        first Interfaces then 'Sorted Ports';
                        action 'Sorted Ports' {
                            first start then 'Collect Owned Elements';
                            action 'Collect Owned Elements';
                            first 'Collect Owned Elements' then 'Sort By Property';
                            action 'Sort By Property';
                            first 'Sort By Property' then done;
                        }
                        first 'Sorted Ports' then done;
                    }
                    first 'Interfaces Table' then done;
                }
                first 'Per Assembly' then 'Filter By Metaclasses 2';
                action 'Filter By Metaclasses 2';
                first 'Filter By Metaclasses 2' then 'Per Activity';
                action 'Per Activity' {
                    first start then Activity;
                    action Activity;
                    first Activity then done;
                }
                first 'Per Activity' then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about final;
            }
            perform action 'interfaces Method' : 'Interfaces Method';
        }
        viewpoint 'Per Part Viewpoint' {
            doc /* One section per part. */
            action def 'Per Part Method' {
                first start then 'Collect Owned Elements';
                action 'Collect Owned Elements';
                first 'Collect Owned Elements' then 'Filter By Metaclasses';
                action 'Filter By Metaclasses';
                first 'Filter By Metaclasses' then 'One Part';
                action 'One Part' : Observatory::Viewpoints::'Per Part Viewpoint'::'One Part';
                first 'One Part' then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about final;
            }
            action def 'One Part' {
                first start then 'The Part';
                action 'The Part';
                first 'The Part' then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about final;
                /* applied stereotype «Dynamic_View»: loop = true; title = One Part */
            }
            perform action 'per Part Method' : 'Per Part Method';
        }
    }
    package Documents {
        part def 'Interface Control Document' {
            view totals :> 'Interface Totals';
            view connections :> Connections;
            view interfaces :> Interfaces;
            view perPart :> 'Per Part';
            /* applied stereotype «Document» */
        }
        view 'Interface Totals' {
            satisfy Observatory::Viewpoints::'Totals Viewpoint';
            expose 'Interface Types'::**;
        }
        view Connections {
            satisfy Observatory::Viewpoints::'Connections Viewpoint';
            expose Observatory::Assemblies::'Observatory System';
        }
        view Interfaces {
            satisfy Observatory::Viewpoints::'Interfaces Viewpoint';
            expose Observatory::Assemblies::'Optical Bench';
        }
        view 'Per Part' {
            satisfy Observatory::Viewpoints::'Per Part Viewpoint';
            expose Observatory::Assemblies::'Optical Bench';
        }
        calc def 'Interface Control Document Interface Totals Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Observatory::Interface Types")),
                            maxDepth = 1),
                        type = ("PartDefinition", "RequirementDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage")),
                    property = "name",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("name"),
                columns = (
                    DocumentQueries::Column(name = "Number of Interfaces", cell = { in row : KerML::Core::Type; DocumentQueries::RelatedElements(source = row, relationshipKind = "typing", direction = "incoming", maxDepth = 1)->SequenceFunctions::size() })))
        }
        calc def 'Interface Control Document Table of Connections Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Observatory System")),
                        maxDepth = 1),
                    type = ("ConnectionUsage", "BindingConnectorAsUsage", "InterfaceUsage", "FlowUsage")),
                properties = ("documentation"),
                columns = (
                    DocumentQueries::Column(name = "Point of Interface on Optical Bench", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->ControlFunctions::select {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::excluding(e.chainingFeature->SequenceFunctions::last())->ControlFunctions::exists {in y : KerML::Core::Feature; y.type.name == "Optical Bench"}}->ControlFunctions::collect {in e1 : KerML::Core::Feature; e1.chainingFeature->SequenceFunctions::excluding(e1.chainingFeature->SequenceFunctions::last())}->ControlFunctions::select {in z : KerML::Core::Feature; z.type.name != "Optical Bench"}.name }),
                    DocumentQueries::Column(name = "Point of Interface on Controller Rack", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->ControlFunctions::select {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::excluding(e.chainingFeature->SequenceFunctions::last())->ControlFunctions::exists {in y : KerML::Core::Feature; y.type.name == "Controller Rack"}}->ControlFunctions::collect {in e1 : KerML::Core::Feature; e1.chainingFeature->SequenceFunctions::excluding(e1.chainingFeature->SequenceFunctions::last())}->ControlFunctions::select {in z : KerML::Core::Feature; z.type.name != "Controller Rack"}.name }),
                    DocumentQueries::Column(name = "Type", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->ControlFunctions::collect {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::last()}.type->DocumentQueries::Distinct() }),
                    DocumentQueries::Column(name = "Direction", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->ControlFunctions::collect {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::last()}.type.member->ControlFunctions::select {in y : KerML::Core::Feature; y.direction->SequenceFunctions::notEmpty()}.direction->DocumentQueries::Distinct() })))
        }
        calc def 'Interface Control Document Assembly Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::collimator")),
                properties = ("name"))
        }
        calc def 'Interface Control Document Interfaces Table Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::RelatedElements(
                    source = DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::collimator")),
                    relationshipKind = ("typing"),
                    direction = ("outgoing"),
                    maxDepth = 1),
                properties = ("name"),
                columns = (
                    DocumentQueries::Column(name = "Description", cell = { in row : KerML::Root::Element; DocumentQueries::WhereType(source = DocumentQueries::Descendants(source = row, maxDepth = 1), type = ("PortUsage")).documentation }),
                    DocumentQueries::Column(name = "Interfaces", cell = { in row : KerML::Root::Element; DocumentQueries::RelatedElements(source = DocumentQueries::WhereType(source = DocumentQueries::Descendants(source = row, maxDepth = 1), type = ("PortUsage")), relationshipKind = ("typing"), direction = ("outgoing"), maxDepth = 1).name })))
        }
        calc def 'Interface Control Document Assembly Rows 2' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::mirror")),
                properties = ("name"))
        }
        calc def 'Interface Control Document Interfaces Table Rows 2' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::RelatedElements(
                    source = DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::mirror")),
                    relationshipKind = ("typing"),
                    direction = ("outgoing"),
                    maxDepth = 1),
                properties = ("name"),
                columns = (
                    DocumentQueries::Column(name = "Description", cell = { in row : KerML::Root::Element; DocumentQueries::WhereType(source = DocumentQueries::Descendants(source = row, maxDepth = 1), type = ("PortUsage")).documentation }),
                    DocumentQueries::Column(name = "Interfaces", cell = { in row : KerML::Root::Element; DocumentQueries::RelatedElements(source = DocumentQueries::WhereType(source = DocumentQueries::Descendants(source = row, maxDepth = 1), type = ("PortUsage")), relationshipKind = ("typing"), direction = ("outgoing"), maxDepth = 1).name })))
        }
        calc def 'Interface Control Document The Part Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::collimator")),
                properties = ("name", "documentation"))
        }
        calc def 'Interface Control Document The Part Rows 2' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::Named(qualifiedName = ("Observatory::Assemblies::Optical Bench::mirror")),
                properties = ("name", "documentation"))
        }
        part def 'Interface Control Document Document' :> DocumentQueries::Document {
            attribute redefines title = "Interface Control Document";
            part 'Interface Totals' : DocumentQueries::Section {
                attribute redefines title = "Interface Totals";
                part table : DocumentQueries::Table {
                    attribute redefines caption = "Interface Totals";
                    calc rows : 'Interface Control Document Interface Totals Rows';
                }
            }
            part Connections : DocumentQueries::Section {
                attribute redefines title = "Connections";
                part table : DocumentQueries::Table {
                    attribute redefines caption = "Table of Connections";
                    calc rows : 'Interface Control Document Table of Connections Rows';
                }
            }
            part Interfaces : DocumentQueries::Section {
                attribute redefines title = "Interfaces";
                part list : DocumentQueries::List {
                    attribute redefines style = "bullet";
                    calc items : 'Interface Control Document Assembly Rows';
                }
                part table : DocumentQueries::Table {
                    attribute redefines caption = "Interfaces Table";
                    calc rows : 'Interface Control Document Interfaces Table Rows';
                }
                part 'list 2' : DocumentQueries::List {
                    attribute redefines style = "bullet";
                    calc items : 'Interface Control Document Assembly Rows 2';
                }
                part 'table 2' : DocumentQueries::Table {
                    attribute redefines caption = "Interfaces Table";
                    calc rows : 'Interface Control Document Interfaces Table Rows 2';
                }
            }
            part 'Per Part' : DocumentQueries::Section {
                attribute redefines title = "Per Part";
                part collimator : DocumentQueries::Section {
                    attribute redefines title = "collimator";
                    part list : DocumentQueries::List {
                        attribute redefines style = "bullet";
                        calc items : 'Interface Control Document The Part Rows';
                    }
                }
                part mirror : DocumentQueries::Section {
                    attribute redefines title = "mirror";
                    part list : DocumentQueries::List {
                        attribute redefines style = "bullet";
                        calc items : 'Interface Control Document The Part Rows 2';
                    }
                }
            }
        }
    }
}
