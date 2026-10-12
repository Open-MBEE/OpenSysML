package MixedReview {
    private import OpenSysMLRenderings::*;

    package P {
        part def Vehicle {
            action def Drive;
            part def Wheel;
            action def Brake;
        }

        use case def A {
            include B;
        }
        use case def B {
            include C;
        }
        use case def C;
    }

    package CaseOwners {
        part def Person;
        part def Vehicle {
            use case def Drive {
                actor driver : Person;
                subject v : Vehicle;
                use case nested;
            }
        }
    }

    package NestedStructures {
        part def Vehicle {
            requirement def R {
                part def Inner;
            }
        }
    }

    requirement def Requirement {
        action a {
            first start;
            then action step;
        }
    }

    view mixed {
        expose P;
        render asMixedDiagram;
    }

    view behaviorView {
        expose Requirement;
        render asMixedDiagram;
    }

    view caseOwnerView {
        expose CaseOwners;
        render asMixedDiagram;
    }

    view transitiveIncludes {
        expose P::A;
        render asMixedDiagram;
    }

    view nestedStructuresView {
        expose NestedStructures;
        render asMixedDiagram;
    }
}
