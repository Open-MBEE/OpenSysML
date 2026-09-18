// Census: validateViewRenderingMembershipOwningType
// Expect: error: 'render' declares the rendering of a view and is only allowed in a view body
package P { part def D; part def Q { render asTable; } }
