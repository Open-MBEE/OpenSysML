package AnonymousPortions {
	attribute def Fuel;
	part def Vehicle {
		attribute fuel : Fuel;
	}
	individual def Car :> Vehicle;
	individual car : Car {
		snapshot :>> start {
			attribute :>> fuel = 100;
		}
		timeslice :>> portionOfLife {
			attribute :>> fuel = 50;
		}
		snapshot :>> done :> fuel {
			attribute :>> fuel = 0;
		}
		snapshot;
		timeslice : Car;
	}
}
