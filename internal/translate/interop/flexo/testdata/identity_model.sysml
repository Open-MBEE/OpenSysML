// The identity-carrying variant of the interop fixture: a ProjectRef scope,
// annotated element ids, and an unannotated sibling, so the measurement says
// whether declared identity survives the stack.
package IdentityInterop {
    @IdentityMetadata::ProjectRef { projectId = "opensysml-interop"; branch = "main"; }

    part def Component {
        @IdentityMetadata::ElementId { id = "0b6e3c1a-4d2f-4e8b-9a7c-5d1f0e9b2a7c"; }
        attribute mass = 0.0;
    }

    part def Battery specializes Component {
        @IdentityMetadata::ElementId { id = "8f3a41d0-0d3e-4c2a-9c3f-2f6a1b5c7d90"; }
        attribute capacity = 1500.0;
    }

    part def Rover {
        part battery : Battery;
    }
}
