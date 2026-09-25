part def 'Blank Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'Blank Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Blank Analysis';
    /* one run performs no behavior: neither Blank Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real = mean;
}
package Results {
    individual part def 'analysis of nothing' :> 'Blank Analysis' {
        /* not migrated: Slot (_firstN) — the slot holds the simulation tool's MonteCarloAnalysis::N: the real 2.5 is not a value of Natural, which the statistic holds */
        attribute :>> t = 2.5;
        analysis 'Monte Carlo' : 'Blank Analysis Monte Carlo' {
            subject :>> analysed : 'analysis of nothing';
        }
    }
    individual part def 'analysis of nothing else' :> 'Blank Analysis' {
        attribute :>> t = 1.25;
    }
    view 'Blank Statistics' {
        expose 'analysis of nothing';
        expose 'analysis of nothing else';
        expose 'Blank Statistics Document';
        render Views::asElementTable;
    }
    calc def 'Blank Statistics Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereFeature(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Results"))),
                    type = ("Blank Analysis")),
                'feature' = "isIndividual",
                operator = "=",
                value = "true"),
            properties = ("name"))
    }
    part def 'Blank Statistics Document' :> DocumentQueries::Document {
        attribute redefines title = "Blank Statistics";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Blank Statistics";
            calc rows : 'Blank Statistics Rows';
        }
    }
}
