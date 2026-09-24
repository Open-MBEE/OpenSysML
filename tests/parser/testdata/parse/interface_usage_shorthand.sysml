// An interface usage may state connector-style ends where its own name would go,
// with or without a body (SysML.xtext InterfaceUsageDeclaration).
package Interfaces {
	port def P;
	part def A {
		port p : P;
	}
	part def Axle {
		part half : A;
	}
	interface def Coupling {
		end port a : P;
		end port b : P;
	}
	part b1 : A;
	part b2 : A;
	part rearAxle : Axle;

	interface b1.p to b2.p {
		doc /* the ends an anonymous interface joins */
	}
	interface b1.p to b2.p;
	interface b1.p to rearAxle.half.p;
	interface [1] b1.p to [1] b2.p;
	interface named : Coupling connect b1.p to b2.p {
		doc /* a named interface states its ends after `connect` */
	}
	interface : Coupling connect b1.p to b2.p;
}
