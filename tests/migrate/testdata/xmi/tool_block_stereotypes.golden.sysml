package Tailoring {
    metadata def Subsystem;
}
package Plant {
    port def iPower;
    part def 'Electron Source';
    part def 'Beam Column' {
        part source : 'Electron Source';
        port 'pwr in' : iPower;
        @MigrationMetadata::AppliedStereotype {
            stereotype = "Subsystem";
        }
    }
    part def Microscope {
        part column : 'Beam Column';
        ref part lab : Laboratory;
        @MigrationMetadata::AppliedStereotype {
            stereotype = "System";
        }
    }
    part def Laboratory {
        @MigrationMetadata::AppliedStereotype {
            stereotype = "External";
        }
    }
    part def Imaging {
        @MigrationMetadata::AppliedStereotype {
            stereotype = "Domain";
        }
    }
    occurrence def Paperwork {
        @Tailoring::Subsystem;
    }
    connection def Feeds {
        end fed : 'Beam Column';
        end feeder : Laboratory;
    }
    view Subsystems {
        expose 'Subsystems Document';
        render Views::asElementTable;
    }
    calc def 'Subsystems Rows' :> DocumentQueries::Query {
        in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
            source = DocumentQueries::Named(qualifiedName = ("Plant")));
        DocumentQueries::Project(
            source = DocumentQueries::Except(
                source = DocumentQueries::WhereType(source = candidates, type = ("PartDefinition", "OccurrenceDefinition")),
                exclude = DocumentQueries::Except(
                    source = DocumentQueries::WhereType(source = candidates, type = ("ItemDefinition")),
                    exclude = DocumentQueries::WhereType(source = candidates, type = ("PartDefinition")))),
            properties = ("name"))
    }
    part def 'Subsystems Document' :> DocumentQueries::Document {
        attribute redefines title = "Subsystems";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Subsystems";
            calc rows : 'Subsystems Rows';
        }
    }
}
