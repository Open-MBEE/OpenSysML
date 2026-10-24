package Demo {
    private import ScalarValues::Real;
    private import Widgets::*;
    private import VendoredParts::*;

    part def Assembly {
        part w : Widget;
        part v : VendoredPart;
        attribute mass : Real = widgetMass + 1.0;
    }
}
