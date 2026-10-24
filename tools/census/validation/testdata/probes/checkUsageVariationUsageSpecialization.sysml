// Census: checkUsageVariationUsageSpecialization
// Expect: error: An attribute must be typed by attribute definitions. variant attribute `x` specializes its owning variation part `v`, which is typed by part def `D`.
package P { part def D; variation part v : D { variant attribute x; } }
