// A requirement usage typed by a definition with no require or assume constraint
// holds vacuously, as the definition does (SysML v2 §9.2.14).
package test {
	requirement def Bare;
	requirement bare : Bare;
}
