// An object is kept by every classifier it is an instance of, its type's
// generalizations included, and dropped by a specialization it is not an
// instance of (KerML 1.1 §8.3.4.9). A union's instances are those of the types
// it unions (KerML 1.0 §8.3.3), so it keeps each of them however deeply nested.
package test {
	part def Vehicle;
	part def Car :> Vehicle;
	part def Truck :> Vehicle;
	part def Wheeled unions Car, Truck;
	part def Registered unions Wheeled;

	part car : Car;
	part truck : Truck;

	calc def InstanceCasts {
		out carAsCar : Car[0..1] = car as Car;
		out carAsVehicle : Vehicle[0..1] = car as Vehicle;
		out carAsTruck : Truck[0..1] = car as Truck;
		out carsOfFleet : Car[0..*] = (car, truck) as Car;
		out vehiclesOfFleet : Vehicle[0..*] = (car, truck) as Vehicle;
		out carAsUnion : Wheeled[0..1] = car as Wheeled;
		out truckAsUnion : Wheeled[0..1] = truck as Wheeled;
		out carAsNestedUnion : Registered[0..1] = car as Registered;
	}

	calc c : InstanceCasts;
}
