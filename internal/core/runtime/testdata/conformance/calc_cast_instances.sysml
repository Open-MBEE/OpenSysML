// An object is kept by every classifier it is an instance of, its type's
// generalizations included, and dropped by a specialization it is not an
// instance of (KerML 1.1 §8.3.4.9).
package test {
	part def Vehicle;
	part def Car :> Vehicle;
	part def Truck :> Vehicle;

	part car : Car;
	part truck : Truck;

	calc def InstanceCasts {
		out carAsCar : Car[0..1] = car as Car;
		out carAsVehicle : Vehicle[0..1] = car as Vehicle;
		out carAsTruck : Truck[0..1] = car as Truck;
		out carsOfFleet : Car[0..*] = (car, truck) as Car;
		out vehiclesOfFleet : Vehicle[0..*] = (car, truck) as Vehicle;
	}

	calc c : InstanceCasts;
}
