// Census: validateMetadataAccessExpressionReferencedElement
// Case: conforming
// Ours: clean
// Pilot: e: An attribute must be typed by attribute definitions. | w: Bound features should have conforming types
package P { part def D; attribute m = D.metadata; }
