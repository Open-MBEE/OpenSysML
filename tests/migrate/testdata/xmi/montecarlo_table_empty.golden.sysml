part def 'Blank Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'Blank Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Blank Analysis';
    /* one run performs no behavior: neither Blank Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real[1] = mean;
}
package Results {
    part 'analysis of nothing' : 'Blank Analysis' {
        attribute :>> t = 2.5;
    }
    part 'analysis of nothing else' : 'Blank Analysis' {
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
            source = DocumentQueries::Except(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Results"))),
                        type = ("Blank Analysis")),
                    type = ("Usage")),
                exclude = DocumentQueries::Descendants(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Union(
                            source = DocumentQueries::Named(qualifiedName = ("Results")),
                            other = DocumentQueries::Descendants(
                                source = DocumentQueries::Union(
                                    source = DocumentQueries::WhereType(
                                        source = DocumentQueries::Named(qualifiedName = ("Results")),
                                        type = ("Package")),
                                    other = DocumentQueries::WhereType(
                                        source = DocumentQueries::Descendants(
                                            source = DocumentQueries::Named(qualifiedName = ("Results"))),
                                        type = ("Package"))),
                                maxDepth = 1)),
                        type = ("Type")))),
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
