// A feature declaration states the types it is a feature of with `featured by`
// (KerML.xtext:569 TypeFeaturingPart, :659 OwnedTypeFeaturing), one target or a
// comma-separated list of them. The clause is written after a name, after a
// multiplicity and after a type; a classifier declaration admits none of it —
// TypeFeaturingPart is a FeatureRelationshipPart, so only a feature states it.
package FeaturedBy {
	class Product;
	class Account;
	class ShoppingCart;

	class Product_Account {
		member feature inCart : ShoppingCart[0..1] featured by Product_Account;
		member feature both : ShoppingCart featured by Product, Account;
		member feature multBeforeType[1..1] : ShoppingCart featured by Product;
		feature bare featured by Product;
	}

	feature onAFeature featured by Product;
}
