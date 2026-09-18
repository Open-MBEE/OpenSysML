package Representations {
	rep asJava language "java"
		/* class Wheel {} */

	language "alf"
		/* WriteLine("anonymous"); */

	part def C {
		attribute x;
		rep inOCL language "ocl"
			/* self.x > 0.0 */

		language "alf"
			/* x = 1; */
	}
}
