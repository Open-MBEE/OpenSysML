// Census: validateSatisfyRequirementUsageReference
// Expect: error: satisfy target must be a requirement usage
package P { part def D { part x : D; satisfy x by x; } }
