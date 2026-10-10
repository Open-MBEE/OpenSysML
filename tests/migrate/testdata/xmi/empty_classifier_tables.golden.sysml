package Operations {
    occurrence def Console;
}
package Tables {
    view Classifiers {
        render Views::asElementTable;
    }
    view Types {
        render Views::asElementTable;
    }
    view Namespaces {
        render Views::asElementTable;
    }
    view 'Packageable Elements' {
        render Views::asElementTable;
    }
    view Actors {
        expose 'Actors Document';
        render Views::asElementTable;
    }
    calc def 'Actors Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Except(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Operations"))),
                    exclude = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Operations")))),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Actors Document' :> DocumentQueries::Document {
        attribute redefines title = "Actors";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Actors";
            calc rows : 'Actors Rows';
        }
    }
    view 'Use Cases' {
        expose 'Use Cases Document';
        render Views::asElementTable;
    }
    calc def 'Use Cases Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Except(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Operations"))),
                    exclude = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Operations")))),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Use Cases Document' :> DocumentQueries::Document {
        attribute redefines title = "Use Cases";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Use Cases";
            calc rows : 'Use Cases Rows';
        }
    }
}
