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
        attribute points : ScalarValues::Real[0..*] ordered nonunique;
    }

    metadata def Canvas {
        doc /* The unit and extent of a view's drawing surface. */
        attribute unit : ScalarValues::String[0..1];
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
    }

    metadata def Style {
        doc /* How the annotated element is drawn: the colours of its fill,
             * outline and text as "#RRGGBB", and the face, size in points and
             * weight of its text. Each is optional; the drawing style supplies
             * what is left unstated. */
        attribute fill : ScalarValues::String[0..1];
        attribute line : ScalarValues::String[0..1];
        attribute text : ScalarValues::String[0..1];
        attribute font : ScalarValues::String[0..1];
        attribute fontSize : ScalarValues::Real[0..1];
        attribute bold : ScalarValues::Boolean[0..1];
        attribute italic : ScalarValues::Boolean[0..1];
    }

    metadata def Note {
        doc /* A note drawn beside the annotated element and anchored to it,
             * or free on the drawing surface when it annotates the view: its
             * text, the top-left corner of its box and an optional fixed
             * size, in the units of Layout. */
        attribute text : ScalarValues::String;
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
    }
}
