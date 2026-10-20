// A verification def and usage the tested part def owns, subject bound to `this` (the owning
// object; `self` would be the case), its three packages in three documents: pass.
package ProductPkg {
	private import DefinitionPkg::Definition;

	part good : Definition {
		attribute :>> cost = 80.0;
	}
}
