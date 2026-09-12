standard library package DiagramLayout {
    doc /* Diagram geometry in notation: where a view draws an element, the
         * waypoints an edge is drawn through and the extent of the drawing
         * surface. Units are pixels, y down, origin at the top left.
         * Non-normative OpenSysML extension, proposed for standardization;
         * see the design record. */

    metadata def Layout {
        doc /* Where the annotated element is drawn: the top-left corner of its
             * box, an optional fixed size, and whether its body is folded. */
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
        attribute collapsed : ScalarValues::Boolean[0..1];
    }

    metadata def Route {
        doc /* The waypoints a connection, transition, succession or flow is
             * drawn through, in order, flattened as x0, y0, x1, y1, ... */
        attribute points : ScalarValues::Real[0..*] ordered;
    }

    metadata def Canvas {
        doc /* The unit and extent of a view's drawing surface. */
        attribute unit : ScalarValues::String[0..1];
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
    }
}
