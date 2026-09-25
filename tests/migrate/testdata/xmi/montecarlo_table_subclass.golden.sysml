part def 'Base Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'Base Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Base Analysis';
    /* one run performs no behavior: neither Base Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real = mean;
}
part def 'Fast Analysis' :> 'Base Analysis';
package Results {
    individual part def 'fast analysis of 9 runs' :> 'Fast Analysis' {
        attribute :>> t = 4.0;
        analysis 'Monte Carlo' : 'Base Analysis Monte Carlo' {
            subject :>> analysed : 'fast analysis of 9 runs';
            out :>> runs = 9;
        }
    }
    view 'Fast Statistics' {
        expose 'fast analysis of 9 runs';
        expose 'Fast Statistics Document';
        render Views::asElementTable;
    }
    calc def 'Fast Statistics Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereFeature(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Results"))),
                        type = ("Fast Analysis")),
                    'feature' = "isIndividual",
                    operator = "=",
                    value = "true"),
                property = "'Monte Carlo'.runs",
                direction = "descending",
                missing = "last",
                multiple = "first"),
            properties = ("name"),
            columns = (
                DocumentQueries::Column(name = "N", expression = 'Monte Carlo'.runs)))
    }
    part def 'Fast Statistics Document' :> DocumentQueries::Document {
        attribute redefines title = "Fast Statistics";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Fast Statistics";
            calc rows : 'Fast Statistics Rows';
        }
    }
}
