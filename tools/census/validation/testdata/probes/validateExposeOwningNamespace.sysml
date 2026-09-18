// Census: validateExposeOwningNamespace
// Expect: error: 'expose' declares what a view usage exposes and is only allowed in a view usage body
package P { part def D; part def E { expose D::*; } }
