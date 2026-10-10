part def Timer {
    attribute t : ScalarValues::Real;
    attribute u : ScalarValues::Real;
    attribute p : ScalarValues::Real default = 0.5;
    perform action tick {
        first start then three;
        action three {
            out result[1] = 3.0;
        }
        action 'set t' {
            in value[1];
            assign t := value;
        }
        first 'set t' then final;
        action final terminate;
        succession flow three.result to 'set t'.value;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
part def 'Timer Analysis' :> Timer;
analysis def 'Timer Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Timer Analysis';
    perform action run ::> analysed.tick;
    attribute :>> observed : ScalarValues::Real = analysed.t;
    return Mean : ScalarValues::Real[1] = mean;
}
part 'timer analysis' : 'Timer Analysis';
part def 'Unbound Analysis' :> Timer;
analysis def 'Unbound Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Unbound Analysis';
    perform action run ::> analysed.tick;
    /* 'Unbound Analysis' inherits MonteCarloAnalysis but binds its Mean to no value of its own, so its statistics summarise no observable, so observed is left unbound */
}
part 'unbound analysis' : 'Unbound Analysis';
action def 'Group 0' {
    @Simulation::Configuration {
        runs = 3;
        draws = Simulation::DrawPolicy::average;
    }
    part target :> 'timer analysis';
    perform action run ::> target.tick;
    /* results of the simulation tool: 11 snapshot(s) in Results standing for 15 run(s) analysing t holding p, t, u */
}
action def 'Group 1' {
    @Simulation::Configuration {
        draws = Simulation::DrawPolicy::average;
    }
    part target :> 'timer analysis';
    perform action run ::> target.tick;
    /* results of the simulation tool: 0 snapshot(s) in Empty analysing t */
}
action def 'Group 2' {
    @Simulation::Configuration {
        runs = 1;
        draws = Simulation::DrawPolicy::average;
    }
    part target :> 'unbound analysis';
    perform action run ::> target.tick;
    /* results of the simulation tool: 1 snapshot(s) in Unbound Results holding t */
}
package Results {
    part 'run 1' : 'Timer Analysis' {
        attribute :>> t = 2.0;
        attribute :>> p = 0.5;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'run 1';
        }
    }
    part 'analysis of 4 runs' : 'Timer Analysis' {
        attribute :>> t = 3.5;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of 4 runs';
            out :>> runs = 4;
            out :>> mean = 3.5;
            out :>> deviation = 0.5;
            out :>> outOfSpec = 1;
        }
    }
    part 'analysis of u' : 'Timer Analysis' {
        attribute :>> u = 9.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of u';
            out :>> runs = 2;
            out :>> mean = 9.0;
        }
    }
    part 'half an analysis' : 'Timer Analysis' {
        attribute :>> t = 6.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'half an analysis';
            out :>> runs = 3;
        }
    }
    part 'analysis without a deviation' : 'Timer Analysis' {
        attribute :>> t = 7.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis without a deviation';
            out :>> runs = 2;
            out :>> mean = 7.0;
        }
    }
    part 'analysis of two and a half runs' : 'Timer Analysis' {
        /* not migrated: Slot (_oddN) — the slot holds the simulation tool's MonteCarloAnalysis::N: the real 2.5 is not a value of Natural, which the statistic holds */
        attribute :>> t = 4.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of two and a half runs';
            out :>> mean = 4.0;
        }
    }
    part 'analysis of more runs than a count holds' : 'Timer Analysis' {
        attribute :>> t = 4.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of more runs than a count holds';
            out :>> runs = 9223372036854775808;
            out :>> mean = 4.0;
        }
    }
    part 'analysis of nothing held' : 'Timer Analysis' {
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis of nothing held';
            out :>> runs = 7;
            out :>> mean = 7.0;
        }
    }
    part 'analysis with a garbled mean' : 'Timer Analysis' {
        /* not migrated: Slot (_garbledMean) — the slot holds the simulation tool's MonteCarloAnalysis::Mean: the string "n/a" is not a value of Real, which the statistic holds */
        /* not migrated: Slot (_garbledDev) — the slot holds the simulation tool's MonteCarloAnalysis::Deviation: real literal "NaN" is not a finite number */
        attribute :>> t = 3.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis with a garbled mean';
            out :>> runs = 3;
        }
    }
    part 'analysis left blank' : 'Timer Analysis' {
        attribute :>> t = 5.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis left blank';
        }
    }
    part 'analysis counted twice' : 'Timer Analysis' {
        /* not migrated: Slot (_twiceN) — the slot holds the simulation tool's MonteCarloAnalysis::N, which 2 slots of the instance hold, and a statistic is one number */
        /* not migrated: Slot (_twiceN2) — the slot holds the simulation tool's MonteCarloAnalysis::N, which 2 slots of the instance hold, and a statistic is one number */
        attribute :>> t = 8.0;
        analysis 'Monte Carlo' : 'Timer Analysis Monte Carlo' {
            subject :>> analysed = 'analysis counted twice';
            out :>> mean = 8.0;
        }
    }
    view 'Statistics Table' {
        expose 'analysis of 4 runs';
        expose 'Statistics Table Document';
        render Views::asElementTable;
    }
    calc def 'Statistics Table Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Except(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Results"))),
                            type = ("Timer Analysis")),
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
                property = "'Monte Carlo'.runs",
                direction = "descending",
                missing = "last",
                multiple = "first"),
            properties = ("name"),
            columns = (
                DocumentQueries::Column(name = "t", cell = { in row : Timer; row.t ?? "" }),
                DocumentQueries::Column(name = "N", path = "'Monte Carlo'.runs"),
                DocumentQueries::Column(name = "Mean", path = "'Monte Carlo'.mean"),
                DocumentQueries::Column(name = "Deviation", path = "'Monte Carlo'.deviation")))
    }
    part def 'Statistics Table Document' :> DocumentQueries::Document {
        attribute redefines title = "Statistics Table";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Statistics Table";
            calc rows : 'Statistics Table Rows';
        }
    }
}
package Empty;
package 'Unbound Results' {
    part 'run 1' : 'Unbound Analysis' {
        attribute :>> t = 1.0;
        analysis 'Monte Carlo' : 'Unbound Analysis Monte Carlo' {
            subject :>> analysed = 'run 1';
            out :>> runs = 5;
        }
    }
}
