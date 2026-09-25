part def 'Sweep Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'Sweep Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Sweep Analysis';
    /* one run performs no behavior: neither Sweep Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real = mean;
}
package Results {
    individual part def 'analysis of 6 runs' :> 'Sweep Analysis' {
        analysis 'Monte Carlo' : 'Sweep Analysis Monte Carlo' {
            subject :>> analysed : 'analysis of 6 runs';
            out :>> runs = 6;
            out :>> mean = 2.5;
            out :>> deviation = 0.75;
            out :>> outOfSpec = 2;
        }
    }
    individual part def 'analysis of 12 runs' :> 'Sweep Analysis' {
        analysis 'Monte Carlo' : 'Sweep Analysis Monte Carlo' {
            subject :>> analysed : 'analysis of 12 runs';
            out :>> runs = 12;
            out :>> mean = 1.25;
        }
    }
    view 'Sweep Statistics' {
        expose 'analysis of 6 runs';
        expose 'analysis of 12 runs';
        expose 'Sweep Statistics Document';
        render Views::asElementTable;
    }
    calc def 'Sweep Statistics Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereFeature(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Results"))),
                        type = ("Sweep Analysis")),
                    'feature' = "isIndividual",
                    operator = "=",
                    value = "true"),
                property = "'Monte Carlo'.mean",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("name"),
            columns = (
                DocumentQueries::Column(name = "N", expression = 'Monte Carlo'.runs),
                DocumentQueries::Column(name = "Mean", expression = 'Monte Carlo'.mean),
                DocumentQueries::Column(name = "Deviation", expression = 'Monte Carlo'.deviation),
                DocumentQueries::Column(name = "OutOfSpec", expression = 'Monte Carlo'.outOfSpec)))
    }
    part def 'Sweep Statistics Document' :> DocumentQueries::Document {
        attribute redefines title = "Sweep Statistics";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Sweep Statistics";
            calc rows : 'Sweep Statistics Rows';
        }
    }
}
