package Optics {
    metadata def Segment;
    metadata def Mount :> Segment;
    metadata def Coupling;
    metadata def Rule;
    metadata def Need;
}
package Bench {
    port def iLight {
        @Optics::Coupling;
    }
    part def Lens {
        @Optics::Segment;
    }
    part def Telescope {
        part 'objective' : Lens;
        port 'light in' : iLight;
        @Optics::Segment;
    }
    part def Tripod {
        @Optics::Mount;
    }
    constraint def 'Focus Rule' {
        @Optics::Rule;
    }
    requirement 'Sharp Image' {
        doc /* The image is sharp. */
        @Optics::Need;
    }
    connection def Carries {
        end carrier : Tripod;
        end carried : Telescope;
    }
    view Segments {
        expose 'Segments Document';
        render Views::asElementTable;
    }
    calc def 'Segments Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereMetadata(
                source = DocumentQueries::Descendants(
                    source = DocumentQueries::Named(qualifiedName = ("Bench"))),
                'metadata' = ("Optics::Segment")),
            properties = ("name"))
    }
    part def 'Segments Document' :> DocumentQueries::Document {
        attribute redefines title = "Segments";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Segments";
            calc rows : 'Segments Rows';
        }
    }
}
