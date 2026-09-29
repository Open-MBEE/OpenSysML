// Invalid: library members are not implicitly visible — only a root namespace's own top-level members resolve unqualified ([SysML, 7.2] over [KerML, 8.2.3.2, 8.2.3.5]). VerificationCases::VerificationMethod and VerificationMethodKind need an import or qualification; the pilot reports "Couldn't resolve reference to Type 'VerificationMethod'".
package S100 {
    verification def T {
        metadata VerificationMethod { kind = VerificationMethodKind::test; }
    }
    verification def U {
        @VerificationMethod { kind = VerificationMethodKind::analyze; }
    }
}
