// A verification def and usage the tested part def owns, subject bound to `this` (the owning
// object; `self` would be the case), its three packages in three documents: fail.
package ProductPkg {
	private import DefinitionPkg::Definition;

	part bad : Definition {
		attribute :>> cost = 120.0;
	}
}
