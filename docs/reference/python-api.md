# The Python API reference

This page documents every name exported by `opensysml.__all__`, grouped by
the module that defines it. For task-oriented help, see [Python client
guides](../clients/python/index.md): [models and symbols](../clients/python/models.md),
[instances and values](../clients/python/instances.md),
[verification and analysis](../clients/python/verification.md),
[editing and saving](../clients/python/editing-and-saving.md),
[queries and documents](../clients/python/queries.md),
[errors](../clients/python/errors.md),
[the service](../clients/python/service.md) and
[typed classes](../clients/python/typed-classes.md).

## opensysml

Top-level constants and functions for loading, evaluating, converting and
connecting to models.

::: opensysml.FORMAT_API_JSON
    options:
      heading_level: 3

::: opensysml.FORMAT_SYSML
    options:
      heading_level: 3

::: opensysml.FORMAT_TURTLE
    options:
      heading_level: 3

::: opensysml.CAPABILITY_CONSTRAINT_BODY_AUTHORING
    options:
      heading_level: 3

::: opensysml.CAPABILITY_STATE_ACTION_AUTHORING
    options:
      heading_level: 3

::: opensysml.CAPABILITY_BIG_INT_VALUES
    options:
      heading_level: 3

::: opensysml.CAPABILITY_RATIONAL_VALUES
    options:
      heading_level: 3

::: opensysml.load
    options:
      heading_level: 3

::: opensysml.loads
    options:
      heading_level: 3

::: opensysml.parse_sources
    options:
      heading_level: 3

::: opensysml.connect
    options:
      heading_level: 3

::: opensysml.convert
    options:
      heading_level: 3

::: opensysml.migrate
    options:
      heading_level: 3

::: opensysml.evaluate
    options:
      heading_level: 3

::: opensysml.instantiate
    options:
      heading_level: 3

::: opensysml.DEFAULT_PORT
    options:
      heading_level: 3

::: opensysml.__version__
    options:
      heading_level: 3

## opensysml.connection

Connections own or reach a service, and resolve their address and release.

::: opensysml.Connection
    options:
      heading_level: 3

::: opensysml.split_target
    options:
      heading_level: 3

## opensysml.model

A parsed model is the entry point for symbol lookup, evaluation and execution.

::: opensysml.Model
    options:
      heading_level: 3

## opensysml.symbol

Symbols represent declarations and their resolved facts in a model.

::: opensysml.Symbol
    options:
      heading_level: 3

## opensysml.diagnostic

Diagnostics report parsing, analysis and execution findings.

::: opensysml.Diagnostic
    options:
      heading_level: 3

## opensysml.enumeration

An enum literal carries its declaration identity and any scalar value.

::: opensysml.EnumLiteral
    options:
      heading_level: 3

## opensysml.instance

An instance exposes feature values decoded from the service response.

::: opensysml.Instance
    options:
      heading_level: 3

## opensysml.typed

The generated-class base supplies a typed view over an `Instance`.

::: opensysml.TypedObject
    options:
      heading_level: 3

## opensysml.typefacts

Static type, multiplicity and specialization facts resolved for symbols.

::: opensysml.TypeFacts
    options:
      heading_level: 3

::: opensysml.Multiplicity
    options:
      heading_level: 3

::: opensysml.Specialization
    options:
      heading_level: 3

::: opensysml.SymbolFacts
    options:
      heading_level: 3

::: opensysml.AttributeFacts
    options:
      heading_level: 3

## opensysml.capabilities

Capability names and the error raised when a service omits required support.

::: opensysml.ServerInfo
    options:
      heading_level: 3

::: opensysml.MissingCapabilityError
    options:
      heading_level: 3

## opensysml.values

Decoded value kinds carried by the service wire format.

::: opensysml.UNSET
    options:
      heading_level: 3

::: opensysml.UnsetType
    options:
      heading_level: 3

::: opensysml.Undetermined
    options:
      heading_level: 3

::: opensysml.Array
    options:
      heading_level: 3

::: opensysml.Vector
    options:
      heading_level: 3

::: opensysml.VectorQuantity
    options:
      heading_level: 3

::: opensysml.MeasurementRef
    options:
      heading_level: 3

::: opensysml.Function
    options:
      heading_level: 3

::: opensysml.Metaobject
    options:
      heading_level: 3

::: opensysml.SetValue
    options:
      heading_level: 3

::: opensysml.TensorQuantity
    options:
      heading_level: 3

::: opensysml.InstanceRef
    options:
      heading_level: 3

::: opensysml.INFINITY
    options:
      heading_level: 3

## opensysml.conversion

Format conversion and migration results, including experimental-feature notices.

::: opensysml.Conversion
    options:
      heading_level: 3

::: opensysml.format_of_path
    options:
      heading_level: 3

::: opensysml.ExperimentalFeatureWarning
    options:
      heading_level: 3

::: opensysml.is_experimental
    options:
      heading_level: 3

::: opensysml.Migration
    options:
      heading_level: 3

::: opensysml.MigrationEntry
    options:
      heading_level: 3

::: opensysml.MigrationReport
    options:
      heading_level: 3

::: opensysml.is_v1
    options:
      heading_level: 3

## opensysml.edit

Typed operations for changing a model's original notation.

::: opensysml.Editor
    options:
      heading_level: 3

::: opensysml.Body
    options:
      heading_level: 3

::: opensysml.EditResult
    options:
      heading_level: 3

::: opensysml.AppliedEdit
    options:
      heading_level: 3

::: opensysml.EditedDocument
    options:
      heading_level: 3

## opensysml.errors

The public exception hierarchy for client, service, model and edit failures.

::: opensysml.Referrer
    options:
      heading_level: 3

::: opensysml.OpenSysMLError
    options:
      heading_level: 3

::: opensysml.AnalysisRunError
    options:
      heading_level: 3

::: opensysml.ChecksumMismatchError
    options:
      heading_level: 3

::: opensysml.ConnectionError
    options:
      heading_level: 3

::: opensysml.ConversionError
    options:
      heading_level: 3

::: opensysml.ExecutionError
    options:
      heading_level: 3

::: opensysml.FeatureValueError
    options:
      heading_level: 3

::: opensysml.MigrationError
    options:
      heading_level: 3

::: opensysml.EditError
    options:
      heading_level: 3

::: opensysml.NoEditsError
    options:
      heading_level: 3

::: opensysml.EditTargetError
    options:
      heading_level: 3

::: opensysml.InvalidEditError
    options:
      heading_level: 3

::: opensysml.RenameReferencedError
    options:
      heading_level: 3

::: opensysml.OverlappingEditsError
    options:
      heading_level: 3

::: opensysml.EditResultError
    options:
      heading_level: 3

::: opensysml.OwnerNotFoundError
    options:
      heading_level: 3

::: opensysml.OwnerNotNamespaceError
    options:
      heading_level: 3

::: opensysml.IllegalMemberKindError
    options:
      heading_level: 3

::: opensysml.MemberNameTakenError
    options:
      heading_level: 3

::: opensysml.DeleteReferencedError
    options:
      heading_level: 3

::: opensysml.OwnerInsideTargetError
    options:
      heading_level: 3

::: opensysml.MoveReferencedError
    options:
      heading_level: 3

::: opensysml.ReferencedElsewhereError
    options:
      heading_level: 3

::: opensysml.InstanceTypeError
    options:
      heading_level: 3

::: opensysml.InvalidRequestError
    options:
      heading_level: 3

::: opensysml.ManifestSignatureError
    options:
      heading_level: 3

::: opensysml.ModelError
    options:
      heading_level: 3

::: opensysml.ModelFileNotFoundError
    options:
      heading_level: 3

::: opensysml.ModelNotFoundError
    options:
      heading_level: 3

::: opensysml.ServiceError
    options:
      heading_level: 3

::: opensysml.ServiceTimeoutError
    options:
      heading_level: 3

::: opensysml.StaleServiceError
    options:
      heading_level: 3

::: opensysml.SymbolNotFoundError
    options:
      heading_level: 3

::: opensysml.TypeMismatchError
    options:
      heading_level: 3

::: opensysml.UnpinnedReleaseError
    options:
      heading_level: 3

::: opensysml.UnsignedReleaseError
    options:
      heading_level: 3

::: opensysml.UnsupportedOperationError
    options:
      heading_level: 3

::: opensysml.UnsupportedValueError
    options:
      heading_level: 3

::: opensysml.WrongKindError
    options:
      heading_level: 3

## opensysml.verdict

Typed outcomes for verification, validation, calculation and analysis.

::: opensysml.Verdict
    options:
      heading_level: 3

::: opensysml.CalcResult
    options:
      heading_level: 3

::: opensysml.AnalysisResult
    options:
      heading_level: 3

::: opensysml.CaseEvaluation
    options:
      heading_level: 3

::: opensysml.SweepRow
    options:
      heading_level: 3

::: opensysml.SweepTable
    options:
      heading_level: 3

::: opensysml.Validation
    options:
      heading_level: 3

::: opensysml.VerificationVerdict
    options:
      heading_level: 3

## opensysml.exploration

Outcomes and search metadata returned by behavior exploration.

::: opensysml.Exploration
    options:
      heading_level: 3

::: opensysml.Outcome
    options:
      heading_level: 3

## opensysml.action_run

Action outputs decoded from a completed run.

::: opensysml.ActionOutputs
    options:
      heading_level: 3

## opensysml.engines

Registered analysis engines and the evidence standing of their answers.

::: opensysml.Bound
    options:
      heading_level: 3

::: opensysml.EngineInfo
    options:
      heading_level: 3

::: opensysml.Standing
    options:
      heading_level: 3

## opensysml.query

Results and errors from the standard query model.

::: opensysml.QueryElement
    options:
      heading_level: 3

::: opensysml.QueryError
    options:
      heading_level: 3

## opensysml.sources

Named file and inline source documents parsed together into one model.

::: opensysml.SourceDocument
    options:
      heading_level: 3

## opensysml.document

Typed query rows, document values and render results.

::: opensysml.DocumentEvent
    options:
      heading_level: 3

::: opensysml.DocumentQueryError
    options:
      heading_level: 3

::: opensysml.DocumentQueryResult
    options:
      heading_level: 3

::: opensysml.DocumentRow
    options:
      heading_level: 3

::: opensysml.DocumentState
    options:
      heading_level: 3

::: opensysml.DocumentVerdict
    options:
      heading_level: 3

::: opensysml.ElementRef
    options:
      heading_level: 3

::: opensysml.ObjectRef
    options:
      heading_level: 3
