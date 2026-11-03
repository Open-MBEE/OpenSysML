library package DiagramLayout {
    doc /* Diagram geometry in notation: where a view draws an element, the
         * waypoints an edge is drawn through and the extent of the drawing
         * surface. Units are pixels, y down, origin at the top left.
         * Non-normative OpenSysML extension, proposed for standardization;
         * see the design record. */

    metadata def Layout {
        doc /* Where the annotated element is drawn: the top-left corner of its
             * box, an optional fixed size, whether its body is folded, and
             * which of its compartments are drawn. Each show… switch names
             * one compartment kind of the box: attributes, parts (part
             * usages), references (ref usages), ports, constraints, items
             * (item and flow features) and actions. Unset means drawn;
             * false means the modeler suppressed that compartment on this
             * symbol, so a renderer omits it rather than clipping it.
             * A Layout that binds only collapsed or show… switches is legal
             * and places nothing: it folds the element, or hides its
             * compartments, where the view lays it out itself. x and y are
             * needed only to place the element and are bound together:
             * binding one without the other is an error, and a size without
             * a place is refused. */
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real[0..1];
        attribute height : ScalarValues::Real[0..1];
        attribute collapsed : ScalarValues::Boolean[0..1];
        attribute showAttributes : ScalarValues::Boolean[0..1];
        attribute showParts : ScalarValues::Boolean[0..1];
        attribute showReferences : ScalarValues::Boolean[0..1];
        attribute showPorts : ScalarValues::Boolean[0..1];
        attribute showConstraints : ScalarValues::Boolean[0..1];
        attribute showItems : ScalarValues::Boolean[0..1];
        attribute showActions : ScalarValues::Boolean[0..1];
    }

    metadata def Route {
        doc /* The waypoints a connection, transition, succession or flow is
             * drawn through, in order, flattened as x0, y0, x1, y1, ...
             * A specialization has no element of its own to annotate: its
             * Route is stated about the specific element and names the
             * general it is drawn to, `ref :>> general : Sup;`, one Route
             * per general. */
        attribute points : ScalarValues::Real[0..*] ordered nonunique;
        ref general : Base::Anything[0..1];
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

    metadata def Picture {
        doc /* A picture drawn on the drawing surface of the view it annotates:
             * the file it is read from, relative to the file the view is
             * written in, the top-left corner and size of the box it fills,
             * in the units of Layout, an optional alternative text, and
             * whether it is drawn over the elements it overlaps rather than
             * under them. */
        attribute location : ScalarValues::String;
        attribute x : ScalarValues::Real;
        attribute y : ScalarValues::Real;
        attribute width : ScalarValues::Real;
        attribute height : ScalarValues::Real;
        attribute alt : ScalarValues::String[0..1];
        attribute above : ScalarValues::Boolean[0..1];
    }
}
