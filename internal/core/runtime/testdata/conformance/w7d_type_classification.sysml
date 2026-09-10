private import ScalarValues::*;

package W7D {
	part def Vehicle;
	part def Car :> Vehicle;
	part def Truck :> Vehicle;
	part def Wheeled unions Car, Truck;
	part def Registered unions Wheeled;
	part def Sys;

	attribute n7 : Integer = 7;
	attribute nat3 : Natural = 3;
	attribute r25 : Real = 2.5;
	attribute str : String = "s";
	attribute seqInt : Integer[*] = (1, 2, 3);
	attribute none : Integer[0..1];
	part car : Car;
	part sys : Sys;

	calc classify {
		return : Boolean[*] nonunique = (
			n7 istype Integer,
			n7 hastype Integer,
			n7 istype Real,
			n7 hastype Real,
			n7 istype Natural,
			nat3 istype Integer,
			nat3 hastype Integer,
			W7D::car hastype Car,
			r25 istype Integer,
			W7D::car hastype Vehicle,
			str istype String,
			n7 istype String,
			3 istype Integer,
			3 istype Real,
			seqInt istype Integer,
			(1, "a") istype Integer,
			W7D::none istype Integer,
			W7D::car istype Car,
			W7D::car istype Vehicle,
			W7D::sys istype Car,
			W7D::car istype Wheeled,
			W7D::car istype Registered,
			W7D::car hastype Wheeled,
			W7D::sys istype Wheeled
		);
	}
}
