part def 'Scoped Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'Scoped Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Scoped Analysis';
    /* one run performs no behavior: neither Scoped Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real = mean;
}
package ResultsA {
    individual part def 'analysis of 6 runs' :> 'Scoped Analysis' {
        attribute :>> t = 2.5;
        analysis 'Monte Carlo' : 'Scoped Analysis Monte Carlo' {
            subject :>> analysed : 'analysis of 6 runs';
            out :>> runs = 6;
        }
    }
}
package ResultsB {
    individual part def 'analysis unrecorded' :> 'Scoped Analysis' {
        attribute :>> t = 1.25;
    }
    view 'Scoped Statistics' {
        expose 'analysis unrecorded';
        expose 'Scoped Statistics Document';
        render Views::asElementTable;
    }
    calc def 'Scoped Statistics Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereFeature(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("ResultsB"))),
                    type = ("Scoped Analysis")),
                'feature' = "isIndividual",
                operator = "=",
                value = "true"),
            properties = ("name"))
    }
    part def 'Scoped Statistics Document' :> DocumentQueries::Document {
        attribute redefines title = "Scoped Statistics";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Scoped Statistics";
            calc rows : 'Scoped Statistics Rows';
        }
    }
}
