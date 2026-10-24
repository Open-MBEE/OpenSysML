package Bridge {
    requirement def LoadRequirement {
        doc /* The bridge shall carry its design load. */
    }
    requirement def SpanRequirement {
        doc /* The bridge shall span the river at the chosen crossing. */
    }
    requirement def SafetyRequirement {
        doc /* The bridge shall be safe for pedestrians and vehicles. */
    }

    requirement bridgeSpecification {
        requirement load : LoadRequirement;
        requirement span : SpanRequirement;
        requirement safety : SafetyRequirement;
    }
}
