part def 'DocGen Analysis' {
    attribute t : ScalarValues::Real;
}
analysis def 'DocGen Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'DocGen Analysis';
    /* one run performs no behavior: neither DocGen Analysis nor any general of it has a classifier behavior */
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real = mean;
}
package ResultsA {
    individual part def 'analysis of 6 runs' :> 'DocGen Analysis' {
        attribute :>> t = 2.5;
        analysis 'Monte Carlo' : 'DocGen Analysis Monte Carlo' {
            subject :>> analysed : 'analysis of 6 runs';
            out :>> mean = 2.5;
        }
    }
}
package ResultsB {
    individual part def 'analysis unrecorded' :> 'DocGen Analysis' {
        attribute :>> t = 1.25;
    }
}
package Viewpoints {
    viewpoint 'Statistics Viewpoint' {
        doc /* List the statistics. */
        action def 'Statistics Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements' {
                out result[0..1];
            }
            first 'Collect Owned Elements' then Statistics;
            action Statistics {
                first start then Name;
                action Name;
                first Name then Mean;
                action Mean;
                first Mean then done;
            }
            first Statistics then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'statistics Method' : 'Statistics Method';
    }
}
package Views {
    view 'Scoped Statistics' {
        satisfy Viewpoints::'Statistics Viewpoint';
        expose ResultsB::**;
    }
    view 'Recorded Statistics' {
        satisfy Viewpoints::'Statistics Viewpoint';
        expose ResultsA::**;
    }
}
package Documents {
    part def 'Statistics Report' {
        view scoped :> Views::'Scoped Statistics';
        view recorded :> Views::'Recorded Statistics';
        /* applied stereotype «Document» */
    }
    calc def 'Statistics Report Statistics Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("ResultsB")),
                maxDepth = 1),
            properties = ("name", "documentation"))
    }
    calc def 'Statistics Report Statistics Rows 2' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("ResultsA")),
                maxDepth = 1),
            properties = ("name", "documentation"),
            columns = (
                DocumentQueries::Column(name = "Mean", expression = 'Monte Carlo'.mean)))
    }
    part def 'Statistics Report Document' :> DocumentQueries::Document {
        attribute redefines title = "Statistics Report";
        part 'Scoped Statistics' : DocumentQueries::Section {
            attribute redefines title = "Scoped Statistics";
            part table : DocumentQueries::Table {
                attribute redefines caption = "Statistics";
                calc rows : 'Statistics Report Statistics Rows';
            }
        }
        part 'Recorded Statistics' : DocumentQueries::Section {
            attribute redefines title = "Recorded Statistics";
            part table : DocumentQueries::Table {
                attribute redefines caption = "Statistics";
                calc rows : 'Statistics Report Statistics Rows 2';
            }
        }
    }
}
