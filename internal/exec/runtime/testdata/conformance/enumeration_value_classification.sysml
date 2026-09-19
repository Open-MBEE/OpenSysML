// An enumeration's enumerated values are the only instances of it (SysML v2
// §8.3.7 EnumerationDefinition), so whether a value is of a scalar-valued
// enumeration is decided by equality with them: `3 istype Level` and `3 @ Level`
// hold, `2 istype Level` does not, `3 as Level` is the value `Level::high` and
// `2 as Level` is empty; a feature typed by the enumeration admits an enumerated
// value (its refusal of any other is the write-conformance rule). `hastype` reads the value's own type alone (KerML
// 1.0 §7.4.9.2): a bare 3 is an Integer and no Level, while a literal — written,
// held by a Level feature, or cast to Level — is a Level and no Integer. A plain
// enumeration classifies by identity with its literals; `5 as Even` stays undecided.
private import ScalarValues::*;

package test {
	enum def Level :> Integer { low = 1; high = 3; }
	enum def Grade :> Real { a = 4.0; b = 3.0; }
	enum def Color { red; green; blue; }
	attribute def Even :> Integer;

	attribute two : Integer = 2;
	attribute three : Integer = 3;
	attribute lvl : Level = Level::high;
	attribute held : Level = three;
	attribute c : Color = Color::red;

	calc classify {
		return : Boolean[*] nonunique = (
			3 istype Level, 2 istype Level, 3 @ Level, 2 @ Level, (1, 2, 3) @ Level,
			three istype Level, two istype Level,
			3 hastype Level, 3 hastype Integer, three hastype Level,
			Level::high hastype Level, Level::high hastype Integer, Level::high istype Integer,
			lvl hastype Level, lvl hastype Integer, lvl istype Integer,
			held hastype Level, (3 as Level) hastype Level, (3 as Level) hastype Integer,
			Level::high == 3, (Level::high + 1) hastype Integer,
			4 istype Grade, 2.5 istype Grade, Grade::a hastype Grade, Grade::a hastype Real,
			3 istype Color, Color::red istype Color, Color::red hastype Color, c hastype Color,
			Color::red istype Level, Level::high istype Color
		);
	}

	attribute threeAsLevel : Level[0..1] = 3 as Level;
	attribute twoAsLevel : Level[0..1] = 2 as Level;
	attribute manyAsLevel : Level[0..*] = (1, 2, 3, 4) as Level;
	attribute fourAsGrade : Grade[0..1] = 4 as Grade;
	attribute redAsColor : Color[0..1] = Color::red as Color;
	attribute redAsLevel : Level[0..1] = Color::red as Level;
	attribute fiveAsEven : Even[0..1] = 5 as Even;
}
