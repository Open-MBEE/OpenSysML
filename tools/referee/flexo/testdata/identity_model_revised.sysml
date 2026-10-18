// identity_model.sysml one edit later: Battery renamed under its annotated id,
// Component's mass changed, Rover gone, Wheel new.
package IdentityInterop {
    @IdentityMetadata::ProjectRef { projectId = "opensysml-interop"; branch = "main"; }

    part def Component {
        @IdentityMetadata::ElementId { id = "0b6e3c1a-4d2f-4e8b-9a7c-5d1f0e9b2a7c"; }
        attribute mass = 1.5;
    }

    part def Cell specializes Component {
        @IdentityMetadata::ElementId { id = "8f3a41d0-0d3e-4c2a-9c3f-2f6a1b5c7d90"; }
        attribute capacity = 1500.0;
    }

    part def Wheel;
}
