// A Rectangle's untyped edges e1..e4 are classified as Line by being bound into
// `edges : Line`; each edge's length reads the enclosing rectangle's feature
// (`Rectangle::length`) or a sibling edge's (`e1.length`); the vertex pairs
// index into the eight vertices.
package test {
    private import ShapeItems::*;
    private import SI::*;

    part def Sheet {
        part rect : Rectangle {
            :>> length = 4 [m];
            :>> width = 3 [m];
        }
    }
}
