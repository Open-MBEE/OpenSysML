// Element identity: how a repository element's id is carried in notation and
// survives notation -> RDF -> notation. Design record:
// docs/project/element-identity-annotations.md.
package OpenSysMLIdentity {
    private import ScalarValues::*;
    private import OpenSysMLArtifacts::*;
    private import OpenSysMLPipeline::*;
    private import OpenSysMLSurfaces::*;

    item def EffectiveId;
    item def IdentityFacts :> SideTable;

    metadata def Identity;

    // The two metadata definitions identity is carried by, shipped as a
    // non-normative library extension rather than surface syntax.
    #Identity part def IdentityLibrary :> CodeUnit {
        attribute :>> goPackage = "internal/core/libs";
        attribute libraryFile : String =
            "internal/core/libs/stdlib/OpenSysML Libraries/IdentityMetadata.sysml";
        attribute elementIdDefinition : String = "IdentityMetadata::ElementId";
        attribute projectRefDefinition : String = "IdentityMetadata::ProjectRef";
        attribute standardNotation : Boolean = true;
        attribute optInPerElement : Boolean = true;
    }

    // Readable, deterministic ids derived from a qualified name, plus the
    // derived ids of memberships (`_om`) and expression nodes (`_p`).
    #Identity part def IdEncoder :> CodeUnit {
        attribute :>> goPackage = "internal/translate/rdf";
        attribute alphabet : String = "[a-zA-Z0-9_-]+";
        attribute derivesFromQualifiedName : Boolean = true;
        attribute reservesNoSuffix : Boolean = true;
    }

    // A side table keyed by symbol: the annotated id when an element carries
    // one, the derived id otherwise, plus its nearest ProjectRef scope.
    #Identity part def IdentityTable :> CodeUnit {
        attribute :>> goPackage = "internal/semantic/identity";
        attribute :>> memoized = false;
        attribute keyedBySymbol : Boolean = true;
        attribute touchesSyntaxTree : Boolean = false;
        attribute scopeEqualityIgnoresBranch : Boolean = true;
        out item facts : IdentityFacts;
        port factsOut : ArtifactPort;
    }

    // Constraint tier: id shape, an enclosing ProjectRef, and uniqueness over
    // the whole generated id space of a project scope, derived ids included.
    #Identity part def IdentityCheck :> ValidationPass {
        attribute :>> level = "constraint";
        attribute checksIdShape : Boolean = true;
        attribute requiresEnclosingProject : Boolean = true;
        attribute checksGeneratedIdSpace : Boolean = true;
        attribute scopedByProject : Boolean = true;
        in item facts : IdentityFacts;
    }

    // Mints IRIs from the effective id, consumes the annotations into
    // identity instead of exporting them, and marks an annotated id so
    // explicitness survives a rename.
    #Identity part def RdfWriter :> CodeUnit {
        attribute :>> goPackage = "internal/translate/export";
        attribute marksDeclaredId : Boolean = true;
        attribute consumesAnnotations : Boolean = true;
        attribute qualifiesMultiScopeIris : Boolean = true;
        attribute refusesScopeCollision : Boolean = true;
        in item facts : IdentityFacts;
        out item graph : RdfGraph;
        port factsIn : ArtifactPort;
        port graphOut : ArtifactPort;
    }

    // Re-materializes the annotations on the way back to notation, including
    // the case where an annotated id equals the derived one.
    #Identity part def RdfReader :> CodeUnit {
        attribute :>> goPackage = "internal/translate/export";
        attribute rematerializesAnnotations : Boolean = true;
        attribute readsProvenanceTriples : Boolean = true;
        attribute preservesDegenerateAnnotation : Boolean = true;
        in item graph : RdfGraph;
        out item text : SourceText;
        port graphIn : ArtifactPort;
        port textOut : ArtifactPort;
    }

    // Measures what survives a live Flexo round trip; the committed report is
    // the evidence the standardization submission cites.
    #Identity part def InteropHarness :> CodeUnit {
        attribute :>> goPackage = "internal/translate/interop/flexo";
        attribute measuresRatherThanAsserts : Boolean = true;
    }

    // A diff keyed by effective id, so a rename is an update of the same
    // repository element rather than a delete plus a create. Ids are minted
    // here, never during parsing or analysis, and only when asked.
    #Identity part def RepositorySync :> CodeUnit {
        attribute :>> goPackage = "internal/translate/interop/reposync";
        attribute implemented : Boolean = true;
        attribute mintsIds : Boolean = true;
        attribute mintsOnlyOnRequest : Boolean = true;
        attribute diffsByEffectiveId : Boolean = true;
        attribute changeKinds : String = "create, update, delete, conflict";
        attribute conflictKinds : String = "missing-id, repository-changed";
        attribute deletesNeedConfirmation : Boolean = true;
        attribute stateFileSuffix : String = ".sync.json";
        in item graph : RdfGraph;
        in item repository : RdfGraph;
    }

    // The command-line side of the sync: one model, one repository (a graph
    // file or a live endpoint); only -sync-apply writes, the rest is a dry run.
    #Identity part def SyncCommand :> CodeUnit {
        attribute :>> goPackage = "cmd/sysml";
        attribute flags : String = "-sync-diff, -sync-apply, -sync-base, -sync-state, -sync-confirm-deletes, -sync-mint-ids, -sync-annotate";
        attribute conflictsFailExit : Boolean = true;
    }

    // The one phase of the identity design that is not built: the notation
    // extension submitted to OMG. Everything before it is implemented.
    #Identity part def IdentityStandardization {
        attribute designRecord : String = "docs/project/element-identity-annotations.md";
        attribute implementedPhases : Integer = 5;
        attribute omgIssueFiled : Boolean = false;
    }

    part def IdentityPath {
        part metadataLibrary : IdentityLibrary;
        part encoder : IdEncoder;
        part facts : IdentityTable;
        part check : IdentityCheck;
        part writer : RdfWriter;
        part reader : RdfReader;
        part harness : InteropHarness;
        part sync : RepositorySync;
        part syncCommand : SyncCommand;
        part standardization : IdentityStandardization;

        port notationIn : ArtifactPort;
        port notationOut : ArtifactPort;

        connection factsToWriter : ArtifactChannel connect facts.factsOut to writer.factsIn;
        connection graphToReader : ArtifactChannel connect writer.graphOut to reader.graphIn;

        flow of IdentityFacts from facts.facts to writer.facts;
        flow of IdentityFacts from facts.facts to check.facts;
        flow of RdfGraph from writer.graph to reader.graph;
        flow of RdfGraph from writer.graph to sync.graph;
    }

    part identity : IdentityPath;

    // The round trip the design turns on: notation in, notation out, and the
    // same ids either side of the graph.
    action def CarryIdentity {
        attribute annotated : Boolean = true;

        in item text : SourceText;
        out item written : SourceText;

        first start;

        action buildFacts;
        action deriveId;
        action validateIds;
        action writeGraph;
        action readGraph;
        action rematerialize;

        succession first start then buildFacts;
        succession first buildFacts then idKind;

        // An element without an annotation falls back to the id derived from
        // its qualified name, and the round trip carries no annotation back.
        decide idKind;
        if annotated then validateIds;
        else deriveId;

        succession first deriveId then writeGraph;
        succession first validateIds then writeGraph;
        succession first writeGraph then readGraph;
        succession first readGraph then rematerialize;
    }

    requirement def IdentitySurvivesTheRoundTrip {
        doc /* An annotated id comes back byte for byte through RDF. */
        subject path : IdentityPath;
        require constraint {
            path.writer.marksDeclaredId and path.reader.rematerializesAnnotations
                and path.reader.preservesDegenerateAnnotation
        }
    }

    requirement def IdSpaceIsCollisionFree {
        doc /* An annotated id colliding with a derived one is an error, since
             * the generators reserve no suffix. */
        subject path : IdentityPath;
        require constraint {
            path.check.checksGeneratedIdSpace and path.encoder.reservesNoSuffix
        }
    }

    requirement def IdentityStaysASideTable {
        doc /* Identity is computed beside the tree, never written into it. */
        subject facts : IdentityTable;
        require constraint {
            facts.keyedBySymbol and not facts.touchesSyntaxTree
        }
    }

    requirement def SyncNeverActsSilently {
        doc /* A sync mints ids and deletes elements only when told to, and
             * a conflict is reported rather than resolved. */
        subject path : IdentityPath;
        require constraint {
            path.sync.implemented and path.sync.mintsOnlyOnRequest
                and path.sync.deletesNeedConfirmation and path.syncCommand.conflictsFailExit
        }
    }

    // The local model is diffed against the repository, keyed by effective id.
    // Nothing here writes to the repository; the change set is the result.
    action def SyncModel {
        attribute hasBase : Boolean = false;
        attribute mintIds : Boolean = false;

        in item graph : RdfGraph;
        in item repository : RdfGraph;

        first start;

        action readScope;
        action loadState;
        action diffByEffectiveId;
        action detectRepositoryChanges;
        action mintUuids;
        action writeBack;
        action reportChangeSet;

        succession first start then readScope;
        succession first readScope then loadState;
        succession first loadState then baseGate;

        // Without the last-seen graph a repository-side change cannot be told
        // from a local one, so it is not called a conflict.
        decide baseGate;
        if hasBase then detectRepositoryChanges;
        else diffByEffectiveId;

        succession first detectRepositoryChanges then diffByEffectiveId;
        succession first diffByEffectiveId then mintGate;

        decide mintGate;
        if mintIds then mintUuids;
        else reportChangeSet;

        succession first mintUuids then writeBack;
        succession first writeBack then reportChangeSet;
    }

    requirement identityRoundTrips : IdentitySurvivesTheRoundTrip {
        subject path = identity;
    }

    requirement idsDoNotCollide : IdSpaceIsCollisionFree {
        subject path = identity;
    }

    requirement identityIsBesideTheTree : IdentityStaysASideTable {
        subject facts = identity.facts;
    }

    requirement syncIsExplicit : SyncNeverActsSilently {
        subject path = identity;
    }

    verification def IdentityGate {
        doc /* go test -run 'TestAnnotatedIDRoundTrips|TestExplicitIDEqualToDerivedIDSurvives'
             * ./internal/translate/export */
        subject writer : RdfWriter;
        objective {
            verify identityRoundTrips;
        }
        return attribute verdict : VerificationCases::VerdictKind;
    }

    verification identityGate : IdentityGate {
        subject writer = identity.writer;
    }
}
