package 'Shop Profile' {
    metadata def TableStructure {
        attribute rows : ScalarValues::Integer;
    }
    metadata def InstanceTable {
        attribute scope : ScalarValues::String;
    }
    metadata def Document;
}
package Shop {
    part def Catalog {
        attribute price : ScalarValues::Real;
        @'Shop Profile'::InstanceTable {
            scope = "Shop";
        }
    }
    part def Ledger {
        @'Shop Profile'::TableStructure {
            rows = 12;
        }
    }
    part def Report {
        attribute a;
        attribute b;
        @'Shop Profile'::Document;
        /* applied stereotype «Document» */
    }
    part def SeasonalCatalog :> Catalog;
    individual part def c1 :> Catalog {
        attribute :>> price = 4.5;
    }
    view 'Catalog Table' {
        expose c1;
        render Views::asElementTable;
    }
    view 'Custom Table' {
        expose Catalog;
        render Views::asElementTable;
    }
    view 'Dangling Scope' {
        render Views::asElementTable;
    }
    /* not migrated: «InstanceTable» 'Dangling Scope' — the scope _nowhere resolves to no element */
    view 'Ambiguous Scope' {
        render Views::asElementTable;
    }
    /* not migrated: «InstanceTable» 'Ambiguous Scope' — the scope _shared names 2 module elements (http://example.com/modules/Warehouse.xmi#_shared, http://example.com/modules/Storefront.xmi#_shared) */
    view 'Bad Sort' {
        expose c1;
        render Views::asElementTable;
    }
    /* not migrated: «InstanceTable» 'Bad Sort' — sort "IColumn:_prop_price^Sideways": not in the form <column>^Asc|Desc; sort "price": not in the form <column>^Asc|Desc */
    view 'No Classifier' {
        render Views::asElementTable;
    }
    /* not migrated: «InstanceTable» 'No Classifier' — the instance table names no classifier */
    view 'Ghost Column' {
        expose Catalog;
        render Views::asElementTable;
    }
    /* not migrated: «DiagramTable» 'Ghost Column' — none of the table's columns reads what a query can; the column IColumn:_no_such_property is omitted: the column IColumn:_no_such_property names no property of the document */
    view 'Broken Matrix' {
        expose Catalog;
        render Views::asElementTable;
    }
    /* not migrated: «DependencyMatrix» 'Broken Matrix' — the unnamed criterion is malformed: not well-formed XML: xmi: XML syntax error on line 4: unexpected EOF */
    view 'Orphan Matrix' {
        render Views::asElementTable;
    }
    /* not migrated: «DependencyMatrix» 'Orphan Matrix' — MatrixFilter: no filter application names the diagram */
    view 'Deep Map' {
        expose Catalog;
        expose SeasonalCatalog;
        render Views::asTreeDiagram;
    }
    /* not migrated: «RelationMap» 'Deep Map' — depth "deep": not a non-negative integer */
    view 'Catalog Map' {
        expose Catalog;
        expose SeasonalCatalog;
        expose 'Catalog Map Document';
        render Views::asTreeDiagram;
    }
    calc def 'Catalog Map Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereType(
                source = DocumentQueries::RelatedElements(
                    source = DocumentQueries::Named(qualifiedName = ("Shop::Catalog")),
                    relationshipKind = "specialization",
                    direction = "incoming",
                    maxDepth = 1),
                type = ("PartDefinition", "RequirementDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage")),
            properties = ("qualifiedName", "@type"))
    }
    part def 'Catalog Map Document' :> DocumentQueries::Document {
        attribute redefines title = "Catalog Map";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Catalog Map";
            calc rows : 'Catalog Map Rows';
        }
    }
}
/* not migrated: «InstanceTable» _no_such_diagram — base_Diagram _no_such_diagram names no diagram of the document */
