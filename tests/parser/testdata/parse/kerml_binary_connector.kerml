// A binary connector names itself only ahead of `from`; without `from` the
// first token is the first end (KerML.xtext:836 BinaryConnectorDeclaration),
// which may carry a crossing multiplicity (:855) or a name referencing its
// feature (:856). `all` may stand ahead of `from` or of the first end.
package BinaryConnector {
	class Engine;
	class Tank { feature main : Engine; feature x; }
	class Trigger;

	class Vehicle {
		feature eng : Engine;
		feature tanks : Tank;
		feature a : Tank;
		feature b;
		feature trigger : Trigger;
		feature transitionLink;

		connector eng to tanks.main;
		connector tanks.main to eng;
		connector [0..1] eng to [1..*] tanks.main;
		connector e ::> eng to tanks.main;
		connector e references eng to tanks.main;
		connector a ::> a.x to b;
		connector [1] bead ::> tanks.x to [0..1] b;
		private connector [0..1] transitionLink to [1..*] trigger;
		connector link from eng to tanks.main;
		connector binary : Links::BinaryLink from eng to tanks.main;
		connector all from eng to tanks.main;
		connector all eng to tanks.main;
		connector from eng to tanks.main;
		connector (eng, tanks.main, trigger);
		connector nary (eng, tanks.main, trigger);
	}
}
