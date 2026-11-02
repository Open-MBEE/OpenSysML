package QVTSDemo {
    private import DocumentQueries::*;
    private import KerML::Root::Element;
    private import Lander::Payload;

    calc def CabinMass :> Query {
        Project(
            source = Named(qualifiedName = "Lander::lander::cabin"),
            properties = ("qualifiedName", "mass")
        )
    }

    calc def EmptySelection :> Query {
        Project(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "StateUsage"
            ),
            properties = ("qualifiedName", "mass")
        )
    }

    calc def MisspelledName :> Query {
        Project(
            source = Named(qualifiedName = "Lander::lander::cabni"),
            properties = ("qualifiedName")
        )
    }

    calc def EmptyPackageMember :> Query {
        Project(
            source = Named(qualifiedName = "Lander::Empty::nothing"),
            properties = ("qualifiedName")
        )
    }

    calc def MissingMassProject :> Query {
        Project(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "PartUsage"
            ),
            properties = ("qualifiedName", "mass")
        )
    }

    calc def MissingMassFilter :> Query {
        WhereFeature(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "PartUsage"
            ),
            feature = "mass",
            operator = ">=",
            value = "0.0"
        )
    }

    calc def UnvaluedMass :> Query {
        Project(
            source = Named(qualifiedName = "Lander::lander::scienceBay"),
            properties = ("qualifiedName", "mass")
        )
    }

    calc def AmbiguousPart :> Query {
        Project(
            source = Named(qualifiedName = "Duplicates::repeated"),
            properties = ("qualifiedName", "mass")
        )
    }

    calc def DoubleMassValue :> Query {
        Project(
            source = Named(qualifiedName = "Duplicates::doubleMass"),
            properties = ("qualifiedName", "mass")
        )
    }

    calc def ToleratedMassProject :> Query {
        Project(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "PartUsage"
            ),
            properties = ("qualifiedName", "mass"),
            tolerate = true
        )
    }

    calc def ToleratedMassFilter :> Query {
        WhereFeature(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "PartUsage"
            ),
            feature = "mass",
            operator = ">=",
            value = "0.0",
            tolerate = true
        )
    }

    calc def MassTable :> Query {
        Project(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "PartUsage",
                exact = true
            ),
            properties = ("qualifiedName", "mass")
        )
    }

    calc def OrderedMasses :> Query {
        OrderBy(
            source = MassTable(),
            property = "mass",
            direction = "ascending",
            missing = "last",
            multiple = "first"
        )
    }

    calc def ExactPartUsages :> Query {
        Project(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "PartUsage",
                exact = true
            ),
            properties = ("qualifiedName")
        )
    }

    calc def MetamodelProperties :> Query {
        Project(
            source = WhereType(
                source = Descendants(source = Named(qualifiedName = "Lander::lander")),
                type = "PartUsage",
                exact = true
            ),
            properties = ("qualifiedName", "elementId", "isComposite", "declaredName")
        )
    }

    calc def ChangedMasses :> Query {
        Differ(
            left = Project(
                source = Named(qualifiedName = ("Lander::lander::cabin", "Lander::lander::engine")),
                properties = ("elementId", "mass")
            ),
            right = Project(
                source = Named(qualifiedName = ("Lander::lander::cabin", "Lander::lander::engine")),
                properties = ("elementId"),
                columns = (Column(name = "mass", cell = { in row : Lander::Payload; row.mass + 1.0 }))
            ),
            key = "elementId",
            column = "mass"
        )
    }

    calc def MisspelledDifferKey :> Query {
        Differ(
            left = Project(
                source = Named(qualifiedName = ("Lander::lander::cabin", "Lander::lander::engine")),
                properties = ("elementId", "mass")
            ),
            right = Project(
                source = Named(qualifiedName = ("Lander::lander::cabin", "Lander::lander::engine")),
                properties = ("elementId", "mass")
            ),
            key = "elementIdd",
            column = "mass"
        )
    }

    part def MassReport :> Document {
        attribute redefines title = "Lander Mass Report";
        part masses : Table {
            attribute redefines caption = "Mass values in the lander";
            calc rows : MassTable;
        }
    }
}
