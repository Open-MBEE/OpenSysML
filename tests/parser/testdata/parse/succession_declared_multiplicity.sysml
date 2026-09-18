// A succession states a declaration of its own, so a multiplicity written
// after a type or a redefinition is the declaration's, not the first end's
// (SysML.xtext:1033 SuccessionAsUsage, UsageDeclaration).
package SuccessionDeclarations {
    connection def Before {
        end source;
        end target;
    }

    action def Base {
        action paint;
        action dry;

        succession named : Before [1] first paint then dry;
    }

    // A redefinition names an inherited feature, so the succession it redefines
    // is declared in the general action, not alongside it.
    action def Process :> Base {
        succession redefines named : Before [1] first paint then dry;
        succession [1] first paint then dry;
        succession first paint then dry;
    }
}
