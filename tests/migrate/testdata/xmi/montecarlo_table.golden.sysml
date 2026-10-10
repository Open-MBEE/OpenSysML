part def 'Sweep Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'Sweep Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Sweep Analysis';
    /* one run performs no behavior: neither Sweep Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real[1] = mean;
}
package Results {
    part 'analysis of 6 runs' : 'Sweep Analysis' {
        analysis 'Monte Carlo' : 'Sweep Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of 6 runs';
            out :>> runs = 6;
            out :>> mean = 2.5;
            out :>> deviation = 0.75;
            out :>> outOfSpec = 2;
        }
    }
    part 'analysis of 12 runs' : 'Sweep Analysis' {
        analysis 'Monte Carlo' : 'Sweep Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of 12 runs';
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
                source = DocumentQueries::Except(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Results"))),
                            type = ("Sweep Analysis")),
                        type = ("Usage")),
                    exclude = DocumentQueries::Descendants(
                        source = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Union(
                                    source = DocumentQueries::Named(qualifiedName = ("Results")),
                                    other = DocumentQueries::WhereType(
                                        source = DocumentQueries::Descendants(
                                            source = DocumentQueries::Named(qualifiedName = ("Results"))),
                                        type = ("Package"))),
                                maxDepth = 1),
                            type = ("Type")))),
                property = "'Monte Carlo'.mean",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("name"),
            columns = (
                DocumentQueries::Column(name = "N", path = "'Monte Carlo'.runs"),
                DocumentQueries::Column(name = "Mean", path = "'Monte Carlo'.mean"),
                DocumentQueries::Column(name = "Deviation", path = "'Monte Carlo'.deviation"),
                DocumentQueries::Column(name = "OutOfSpec", path = "'Monte Carlo'.outOfSpec")))
    }
    part def 'Sweep Statistics Document' :> DocumentQueries::Document {
        attribute redefines title = "Sweep Statistics";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Sweep Statistics";
            calc rows : 'Sweep Statistics Rows';
        }
    }
}
