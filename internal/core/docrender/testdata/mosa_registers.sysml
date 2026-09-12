package Registers {
	private import MOSA::*;
	private import DocumentQueries::*;

	part def Vehicle;

	part vehicle : Vehicle {
		#modularSystemInterface interface shallow;
		part n1 { part n2 { part n3 { part n4 { part n5 { part n6 { part n7 { part n8 {
			part n9 { part n10 { part n11 { part n12 { part n13 { part n14 { part n15 { part n16 {
				#modularSystemInterface interface deep;
			} } } } } } } }
		} } } } } } } }
	}

	part def RegisterReport :> Document {
		attribute redefines title = "Register Depth";

		part defaulted : Table {
			attribute redefines caption = "Sixteen levels";
			calc rows : InterfaceRegister {
				in :>> root = vehicle;
			}
		}

		part deepened : Table {
			attribute redefines caption = "Twenty levels";
			calc rows : InterfaceRegister {
				in :>> root = vehicle;
				in :>> depth = 20;
			}
		}
	}
}
