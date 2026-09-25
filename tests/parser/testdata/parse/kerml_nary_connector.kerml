// A connector declaration may state its ends as a parenthesized list of two or
// more (KerML.xtext:842 NaryConnectorDeclaration), each optionally carrying a
// crossing multiplicity (:851 OwnedCrossingMultiplicityMember).
package NaryConnector {
	class Cart;
	class Product;
	class Account;

	assoc ProductSelection {
		end feature cart : Cart;
		end feature product : Product;
		end feature account : Account;
	}

	class Shop {
		feature myCart : Cart;
		feature products : Product;
		feature myAccount : Account;

		connector ps1 : ProductSelection (myCart, products, myAccount);
		connector ps2 : ProductSelection ([1] myCart, [0..1] products, [1] myAccount);
		connector (myCart, products);
	}
}
