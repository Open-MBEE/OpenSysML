part def 'Scoped Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'Scoped Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Scoped Analysis';
    /* one run performs no behavior: neither Scoped Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real[1] = mean;
}
package ResultsA {
    part 'analysis of 6 runs' : 'Scoped Analysis' {
        attribute :>> t = 2.5;
        analysis 'Monte Carlo' : 'Scoped Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of 6 runs';
            out :>> runs = 6;
        }
    }
}
package ResultsB {
    part 'analysis unrecorded' : 'Scoped Analysis' {
        attribute :>> t = 1.25;
    }
    view 'Scoped Statistics' {
        expose 'analysis unrecorded';
        expose 'Scoped Statistics Document';
        render Views::asElementTable;
    }
    calc def 'Scoped Statistics Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Except(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("ResultsB"))),
                        type = ("Scoped Analysis")),
                    type = ("Usage")),
                exclude = DocumentQueries::Descendants(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Union(
                                source = DocumentQueries::Named(qualifiedName = ("ResultsB")),
                                other = DocumentQueries::WhereType(
                                    source = DocumentQueries::Descendants(
                                        source = DocumentQueries::Named(qualifiedName = ("ResultsB"))),
                                    type = ("Package"))),
                            maxDepth = 1),
                        type = ("Type")))),
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
