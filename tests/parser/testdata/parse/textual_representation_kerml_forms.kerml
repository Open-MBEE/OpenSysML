package Representations {
	rep asJava language "java"
		/* class Wheel {} */

	language "alf"
		/* WriteLine("anonymous"); */

	rep <ocl> inOCL language "ocl"
		/* self.x > 0.0 */

	behavior setX {
		in newX;
		language "alf"
			/* x = newX; */
	}
}
