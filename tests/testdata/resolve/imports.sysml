package ImportsA { part def Engine; }
package ImportsB { part def Engine; }
package ImportsQ { public import ImportsA::*; }
package ImportsC {
	private import ImportsA::*;
	private import ImportsB::*;
	part e : Engine;
}
package ImportsD { private import ImportsA::*; private import ImportsQ::*; }
package ImportsE { private import ImportsA::*; private import ImportsB::*; part def Engine; }
package ImportsF { private import ImportsA::Engine; private import ImportsA::*; }
package ImportsV { package ImportsP { part vb; } package ImportsR { part vb; } }
package ImportsG { private import ImportsV::**; }
