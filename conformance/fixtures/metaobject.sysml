package Meta {
	private import ScalarValues::*;
	metadata def Safety { attribute level : Integer = 2; }
	part def Vehicle { attribute mass : Real; }
	part seatBelt : Vehicle { @Safety { level = 4; } }
	attribute asFeature [*] = seatBelt meta KerML::Feature;
	attribute everything [*] = seatBelt.metadata;
	attribute notADefinition [*] = seatBelt meta SysML::PartDefinition;
	calc def SameElement { in m : KerML::Feature; return : Boolean = m === (seatBelt meta KerML::Type)#(1); }
	calc sameElement : SameElement;
	calc def NameOf { in m : KerML::Feature; return : String = m.declaredName; }
	calc nameOf : NameOf;
}
