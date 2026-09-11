// `istype`, `hastype`, `@` and `as` judge a value by the types it is a value of
// (KerML 1.0 §8.3.4.9, §8.3.4.10, §8.3.3): an integer is an Integer and so a
// Rational and Real; a finite real is a Rational (§8.4.4.9.2) whole or not, never
// an Integer; a quotient is a Rational (IntegerFunctions::'/'); a quantity is a
// ScalarQuantityValue and, through the feature holding it, a LengthValue; an
// enumeration literal, an object and a sequence are of their own types; an empty
// value of every type; `*` is a Positive (§8.4.4.6) and so a Natural and Integer.
// `hastype` is the direct type alone; a cast keeps what `istype` affirms and
// cannot decide `5 as Even` or `* as Boundless`, while a feature declared of
// either subtype holds the value.
private import ScalarValues::*;
private import ISQ::*;
private import SI::*;
private import Quantities::*;

package test {
	enum def Color { red; green; }
	part def Vehicle;
	part def Car :> Vehicle;
	attribute def Even :> Integer;
	attribute def Boundless :> Positive;

	attribute n : Integer = 4;
	attribute r : Real = 4.0;
	attribute q : Rational = 6 / 3;
	attribute len : LengthValue = 4 [SI::m];
	attribute c : Color = Color::red;
	part car : Car;
	attribute seq : Integer[*] = (1, 2, 3);
	attribute mixed : Real[*] = (1, 2.5, 3);
	attribute none : Integer[0..1];
	attribute e : Even = 4;
	attribute limit : Boundless = *;

	calc classify {
		return : Boolean[*] nonunique = (
			n istype Integer, n hastype Integer, n @ Integer,
			n istype Rational, n hastype Rational, n istype Real, n istype Natural,
			r istype Integer, r hastype Integer, r @ Integer,
			r istype Rational, r hastype Rational, r istype Real, r hastype Real,
			q istype Integer, q hastype Integer, q @ Integer, q istype Rational, q istype Real,
			len istype LengthValue, len hastype LengthValue, len @ LengthValue,
			len istype ScalarQuantityValue, len hastype ScalarQuantityValue, len istype MassValue,
			c istype Color, c hastype Color, c @ Color, c istype Integer,
			car istype Car, car hastype Car, car istype Vehicle, car hastype Vehicle, car @ Vehicle,
			seq istype Integer, seq @ Integer, seq istype Real, seq hastype Real,
			mixed istype Real, mixed istype Integer, mixed @ Integer,
			none istype Integer, none hastype Integer, none @ Integer,
			e istype Even, e hastype Even, e istype Integer, e hastype Integer,
			limit istype Boundless, limit hastype Boundless, limit @ Boundless,
			limit istype Positive, limit hastype Positive, limit istype Integer, limit istype String,
			* istype Boundless, * hastype Boundless, * istype Natural
		);
	}

	attribute nAsInteger : Integer[0..1] = n as Integer;
	attribute nAsReal : Real[0..1] = n as Real;
	attribute rAsInteger : Integer[0..1] = r as Integer;
	attribute rAsRational : Rational[0..1] = r as Rational;
	attribute qAsInteger : Integer[0..1] = q as Integer;
	attribute qAsRational : Rational[0..1] = q as Rational;
	attribute lenAsLength : LengthValue[0..1] = len as LengthValue;
	attribute lenAsMass : MassValue[0..1] = len as ScalarQuantityValue as MassValue;
	attribute cAsColor : Color[0..1] = c as Color;
	part carAsVehicle : Vehicle[0..1] = car as Vehicle;
	attribute seqAsInteger : Integer[0..*] = seq as Integer;
	attribute mixedAsInteger : Integer[0..*] = mixed as Integer;
	attribute noneAsInteger : Integer[0..1] = none as Integer;
	attribute eAsEven : Even[0..1] = e as Even;
	attribute fiveAsEven : Even[0..1] = 5 as Even;
	attribute limitAsBoundless : Boundless[0..1] = limit as Boundless;
	attribute limitAsInteger : Integer[0..1] = limit as Integer;
	attribute unboundedAsString : String[0..1] = * as String;
	attribute unboundedAsBoundless : Boundless[0..1] = * as Boundless;
}
