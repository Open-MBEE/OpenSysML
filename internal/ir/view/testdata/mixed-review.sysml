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
        use case def B;
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
}
