from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class FailureReason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    FAILURE_REASON_UNSPECIFIED: _ClassVar[FailureReason]
    FAILURE_REASON_EVALUATION: _ClassVar[FailureReason]
    FAILURE_REASON_WRONG_KIND: _ClassVar[FailureReason]
    FAILURE_REASON_AMBIGUOUS_SUBJECT: _ClassVar[FailureReason]

class EditFailure(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    EDIT_FAILURE_UNSPECIFIED: _ClassVar[EditFailure]
    EDIT_FAILURE_NO_OPERATIONS: _ClassVar[EditFailure]
    EDIT_FAILURE_UNKNOWN_TARGET: _ClassVar[EditFailure]
    EDIT_FAILURE_AMBIGUOUS_TARGET: _ClassVar[EditFailure]
    EDIT_FAILURE_NOT_VALUED: _ClassVar[EditFailure]
    EDIT_FAILURE_INVALID_VALUE: _ClassVar[EditFailure]
    EDIT_FAILURE_INVALID_NAME: _ClassVar[EditFailure]
    EDIT_FAILURE_NOT_NAMED: _ClassVar[EditFailure]
    EDIT_FAILURE_RENAME_REFERENCED: _ClassVar[EditFailure]
    EDIT_FAILURE_OVERLAPPING_EDITS: _ClassVar[EditFailure]
    EDIT_FAILURE_RESULT_INVALID: _ClassVar[EditFailure]
    EDIT_FAILURE_OWNER_UNKNOWN: _ClassVar[EditFailure]
    EDIT_FAILURE_OWNER_NOT_NAMESPACE: _ClassVar[EditFailure]
    EDIT_FAILURE_ILLEGAL_KIND: _ClassVar[EditFailure]
    EDIT_FAILURE_MEMBER_NAME_TAKEN: _ClassVar[EditFailure]
    EDIT_FAILURE_DELETE_REFERENCED: _ClassVar[EditFailure]

class PrimitiveOperator(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PRIMITIVE_OPERATOR_UNSPECIFIED: _ClassVar[PrimitiveOperator]
    PRIMITIVE_OPERATOR_EQUAL: _ClassVar[PrimitiveOperator]
    PRIMITIVE_OPERATOR_GREATER: _ClassVar[PrimitiveOperator]
    PRIMITIVE_OPERATOR_LESS: _ClassVar[PrimitiveOperator]

class CompositeOperator(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COMPOSITE_OPERATOR_UNSPECIFIED: _ClassVar[CompositeOperator]
    COMPOSITE_OPERATOR_AND: _ClassVar[CompositeOperator]
    COMPOSITE_OPERATOR_OR: _ClassVar[CompositeOperator]
FAILURE_REASON_UNSPECIFIED: FailureReason
FAILURE_REASON_EVALUATION: FailureReason
FAILURE_REASON_WRONG_KIND: FailureReason
FAILURE_REASON_AMBIGUOUS_SUBJECT: FailureReason
EDIT_FAILURE_UNSPECIFIED: EditFailure
EDIT_FAILURE_NO_OPERATIONS: EditFailure
EDIT_FAILURE_UNKNOWN_TARGET: EditFailure
EDIT_FAILURE_AMBIGUOUS_TARGET: EditFailure
EDIT_FAILURE_NOT_VALUED: EditFailure
EDIT_FAILURE_INVALID_VALUE: EditFailure
EDIT_FAILURE_INVALID_NAME: EditFailure
EDIT_FAILURE_NOT_NAMED: EditFailure
EDIT_FAILURE_RENAME_REFERENCED: EditFailure
EDIT_FAILURE_OVERLAPPING_EDITS: EditFailure
EDIT_FAILURE_RESULT_INVALID: EditFailure
EDIT_FAILURE_OWNER_UNKNOWN: EditFailure
EDIT_FAILURE_OWNER_NOT_NAMESPACE: EditFailure
EDIT_FAILURE_ILLEGAL_KIND: EditFailure
EDIT_FAILURE_MEMBER_NAME_TAKEN: EditFailure
EDIT_FAILURE_DELETE_REFERENCED: EditFailure
PRIMITIVE_OPERATOR_UNSPECIFIED: PrimitiveOperator
PRIMITIVE_OPERATOR_EQUAL: PrimitiveOperator
PRIMITIVE_OPERATOR_GREATER: PrimitiveOperator
PRIMITIVE_OPERATOR_LESS: PrimitiveOperator
COMPOSITE_OPERATOR_UNSPECIFIED: CompositeOperator
COMPOSITE_OPERATOR_AND: CompositeOperator
COMPOSITE_OPERATOR_OR: CompositeOperator

class Verdict(_message.Message):
    __slots__ = ("kind", "element_id", "element", "holds", "condition", "instance_id", "instance_type_id", "error", "failure_reason", "requirement_id", "engine", "strength", "bounds")
    KIND_FIELD_NUMBER: _ClassVar[int]
    ELEMENT_ID_FIELD_NUMBER: _ClassVar[int]
    ELEMENT_FIELD_NUMBER: _ClassVar[int]
    HOLDS_FIELD_NUMBER: _ClassVar[int]
    CONDITION_FIELD_NUMBER: _ClassVar[int]
    INSTANCE_ID_FIELD_NUMBER: _ClassVar[int]
    INSTANCE_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    FAILURE_REASON_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENT_ID_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    STRENGTH_FIELD_NUMBER: _ClassVar[int]
    BOUNDS_FIELD_NUMBER: _ClassVar[int]
    kind: str
    element_id: str
    element: str
    holds: bool
    condition: str
    instance_id: int
    instance_type_id: str
    error: str
    failure_reason: FailureReason
    requirement_id: str
    engine: str
    strength: str
    bounds: _containers.RepeatedCompositeFieldContainer[Bound]
    def __init__(self, kind: _Optional[str] = ..., element_id: _Optional[str] = ..., element: _Optional[str] = ..., holds: _Optional[bool] = ..., condition: _Optional[str] = ..., instance_id: _Optional[int] = ..., instance_type_id: _Optional[str] = ..., error: _Optional[str] = ..., failure_reason: _Optional[_Union[FailureReason, str]] = ..., requirement_id: _Optional[str] = ..., engine: _Optional[str] = ..., strength: _Optional[str] = ..., bounds: _Optional[_Iterable[_Union[Bound, _Mapping]]] = ...) -> None: ...

class Bound(_message.Message):
    __slots__ = ("name", "limit", "reached")
    NAME_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    REACHED_FIELD_NUMBER: _ClassVar[int]
    name: str
    limit: int
    reached: bool
    def __init__(self, name: _Optional[str] = ..., limit: _Optional[int] = ..., reached: _Optional[bool] = ...) -> None: ...

class VerifyConstraintRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id", "subject_symbol_id", "engine")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    subject_symbol_id: str
    engine: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ..., subject_symbol_id: _Optional[str] = ..., engine: _Optional[str] = ...) -> None: ...

class VerifyConstraintResponse(_message.Message):
    __slots__ = ("verdict", "instances", "error", "diagnostics")
    VERDICT_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    verdict: Verdict
    instances: _containers.RepeatedCompositeFieldContainer[Instance]
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    def __init__(self, verdict: _Optional[_Union[Verdict, _Mapping]] = ..., instances: _Optional[_Iterable[_Union[Instance, _Mapping]]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ...) -> None: ...

class VerifyRequirementRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id", "subject_symbol_id", "engine")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    subject_symbol_id: str
    engine: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ..., subject_symbol_id: _Optional[str] = ..., engine: _Optional[str] = ...) -> None: ...

class VerificationVerdict(_message.Message):
    __slots__ = ("case_id", "kind", "detail", "subcase", "requirement_id")
    CASE_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    SUBCASE_FIELD_NUMBER: _ClassVar[int]
    REQUIREMENT_ID_FIELD_NUMBER: _ClassVar[int]
    case_id: str
    kind: str
    detail: str
    subcase: bool
    requirement_id: str
    def __init__(self, case_id: _Optional[str] = ..., kind: _Optional[str] = ..., detail: _Optional[str] = ..., subcase: _Optional[bool] = ..., requirement_id: _Optional[str] = ...) -> None: ...

class VerifyRequirementResponse(_message.Message):
    __slots__ = ("verdict", "instances", "error", "diagnostics", "verification_verdicts")
    VERDICT_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    VERIFICATION_VERDICTS_FIELD_NUMBER: _ClassVar[int]
    verdict: Verdict
    instances: _containers.RepeatedCompositeFieldContainer[Instance]
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    verification_verdicts: _containers.RepeatedCompositeFieldContainer[VerificationVerdict]
    def __init__(self, verdict: _Optional[_Union[Verdict, _Mapping]] = ..., instances: _Optional[_Iterable[_Union[Instance, _Mapping]]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., verification_verdicts: _Optional[_Iterable[_Union[VerificationVerdict, _Mapping]]] = ...) -> None: ...

class VerifySatisfactionRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id", "engine")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    engine: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ..., engine: _Optional[str] = ...) -> None: ...

class VerifySatisfactionResponse(_message.Message):
    __slots__ = ("verdicts", "instances", "error", "diagnostics", "failure_reason", "verification_verdicts")
    VERDICTS_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    FAILURE_REASON_FIELD_NUMBER: _ClassVar[int]
    VERIFICATION_VERDICTS_FIELD_NUMBER: _ClassVar[int]
    verdicts: _containers.RepeatedCompositeFieldContainer[Verdict]
    instances: _containers.RepeatedCompositeFieldContainer[Instance]
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    failure_reason: FailureReason
    verification_verdicts: _containers.RepeatedCompositeFieldContainer[VerificationVerdict]
    def __init__(self, verdicts: _Optional[_Iterable[_Union[Verdict, _Mapping]]] = ..., instances: _Optional[_Iterable[_Union[Instance, _Mapping]]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., failure_reason: _Optional[_Union[FailureReason, str]] = ..., verification_verdicts: _Optional[_Iterable[_Union[VerificationVerdict, _Mapping]]] = ...) -> None: ...

class EvaluateCalcRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id", "arguments", "engine")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    arguments: _containers.RepeatedCompositeFieldContainer[Value]
    engine: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ..., arguments: _Optional[_Iterable[_Union[Value, _Mapping]]] = ..., engine: _Optional[str] = ...) -> None: ...

class EvaluateCalcResponse(_message.Message):
    __slots__ = ("result", "outputs", "error", "diagnostics", "failure_reason", "engine", "strength", "bounds")
    RESULT_FIELD_NUMBER: _ClassVar[int]
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    FAILURE_REASON_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    STRENGTH_FIELD_NUMBER: _ClassVar[int]
    BOUNDS_FIELD_NUMBER: _ClassVar[int]
    result: Value
    outputs: _containers.RepeatedCompositeFieldContainer[CalcOutput]
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    failure_reason: FailureReason
    engine: str
    strength: str
    bounds: _containers.RepeatedCompositeFieldContainer[Bound]
    def __init__(self, result: _Optional[_Union[Value, _Mapping]] = ..., outputs: _Optional[_Iterable[_Union[CalcOutput, _Mapping]]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., failure_reason: _Optional[_Union[FailureReason, str]] = ..., engine: _Optional[str] = ..., strength: _Optional[str] = ..., bounds: _Optional[_Iterable[_Union[Bound, _Mapping]]] = ...) -> None: ...

class CalcOutput(_message.Message):
    __slots__ = ("name", "value")
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    name: str
    value: Value
    def __init__(self, name: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...

class CaseEvaluation(_message.Message):
    __slots__ = ("function_id", "arguments", "result", "error", "selected", "tied")
    FUNCTION_ID_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    SELECTED_FIELD_NUMBER: _ClassVar[int]
    TIED_FIELD_NUMBER: _ClassVar[int]
    function_id: str
    arguments: _containers.RepeatedCompositeFieldContainer[Value]
    result: Value
    error: str
    selected: bool
    tied: bool
    def __init__(self, function_id: _Optional[str] = ..., arguments: _Optional[_Iterable[_Union[Value, _Mapping]]] = ..., result: _Optional[_Union[Value, _Mapping]] = ..., error: _Optional[str] = ..., selected: _Optional[bool] = ..., tied: _Optional[bool] = ...) -> None: ...

class RunAnalysisRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id", "subject_symbol_id", "arguments", "named_arguments", "schedule", "engine")
    class NamedArgumentsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Value
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    NAMED_ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    subject_symbol_id: str
    arguments: _containers.RepeatedCompositeFieldContainer[Value]
    named_arguments: _containers.MessageMap[str, Value]
    schedule: str
    engine: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ..., subject_symbol_id: _Optional[str] = ..., arguments: _Optional[_Iterable[_Union[Value, _Mapping]]] = ..., named_arguments: _Optional[_Mapping[str, Value]] = ..., schedule: _Optional[str] = ..., engine: _Optional[str] = ...) -> None: ...

class RunAnalysisResponse(_message.Message):
    __slots__ = ("outputs", "verdicts", "instances", "error", "diagnostics", "failure_reason", "verification_verdicts", "outcomes", "exploration", "evaluations", "engine", "strength", "bounds")
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    VERDICTS_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    FAILURE_REASON_FIELD_NUMBER: _ClassVar[int]
    VERIFICATION_VERDICTS_FIELD_NUMBER: _ClassVar[int]
    OUTCOMES_FIELD_NUMBER: _ClassVar[int]
    EXPLORATION_FIELD_NUMBER: _ClassVar[int]
    EVALUATIONS_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    STRENGTH_FIELD_NUMBER: _ClassVar[int]
    BOUNDS_FIELD_NUMBER: _ClassVar[int]
    outputs: _containers.RepeatedCompositeFieldContainer[CalcOutput]
    verdicts: _containers.RepeatedCompositeFieldContainer[Verdict]
    instances: _containers.RepeatedCompositeFieldContainer[Instance]
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    failure_reason: FailureReason
    verification_verdicts: _containers.RepeatedCompositeFieldContainer[VerificationVerdict]
    outcomes: _containers.RepeatedCompositeFieldContainer[Outcome]
    exploration: ExplorationStatus
    evaluations: _containers.RepeatedCompositeFieldContainer[CaseEvaluation]
    engine: str
    strength: str
    bounds: _containers.RepeatedCompositeFieldContainer[Bound]
    def __init__(self, outputs: _Optional[_Iterable[_Union[CalcOutput, _Mapping]]] = ..., verdicts: _Optional[_Iterable[_Union[Verdict, _Mapping]]] = ..., instances: _Optional[_Iterable[_Union[Instance, _Mapping]]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., failure_reason: _Optional[_Union[FailureReason, str]] = ..., verification_verdicts: _Optional[_Iterable[_Union[VerificationVerdict, _Mapping]]] = ..., outcomes: _Optional[_Iterable[_Union[Outcome, _Mapping]]] = ..., exploration: _Optional[_Union[ExplorationStatus, _Mapping]] = ..., evaluations: _Optional[_Iterable[_Union[CaseEvaluation, _Mapping]]] = ..., engine: _Optional[str] = ..., strength: _Optional[str] = ..., bounds: _Optional[_Iterable[_Union[Bound, _Mapping]]] = ...) -> None: ...

class Outcome(_message.Message):
    __slots__ = ("outputs", "final_state", "states_visited", "error", "linearizations", "witness", "diagnostics")
    class OutputsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Value
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    FINAL_STATE_FIELD_NUMBER: _ClassVar[int]
    STATES_VISITED_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    LINEARIZATIONS_FIELD_NUMBER: _ClassVar[int]
    WITNESS_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    outputs: _containers.MessageMap[str, Value]
    final_state: str
    states_visited: _containers.RepeatedScalarFieldContainer[str]
    error: str
    linearizations: int
    witness: _containers.RepeatedScalarFieldContainer[str]
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    def __init__(self, outputs: _Optional[_Mapping[str, Value]] = ..., final_state: _Optional[str] = ..., states_visited: _Optional[_Iterable[str]] = ..., error: _Optional[str] = ..., linearizations: _Optional[int] = ..., witness: _Optional[_Iterable[str]] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ...) -> None: ...

class ExplorationStatus(_message.Message):
    __slots__ = ("complete", "runs", "budgets_hit", "runs_budget", "depth_budget")
    COMPLETE_FIELD_NUMBER: _ClassVar[int]
    RUNS_FIELD_NUMBER: _ClassVar[int]
    BUDGETS_HIT_FIELD_NUMBER: _ClassVar[int]
    RUNS_BUDGET_FIELD_NUMBER: _ClassVar[int]
    DEPTH_BUDGET_FIELD_NUMBER: _ClassVar[int]
    complete: bool
    runs: int
    budgets_hit: _containers.RepeatedScalarFieldContainer[str]
    runs_budget: int
    depth_budget: int
    def __init__(self, complete: _Optional[bool] = ..., runs: _Optional[int] = ..., budgets_hit: _Optional[_Iterable[str]] = ..., runs_budget: _Optional[int] = ..., depth_budget: _Optional[int] = ...) -> None: ...

class ListEnginesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class EngineInfo(_message.Message):
    __slots__ = ("name", "authority", "answers", "bounds", "process", "process_found", "ready", "unavailable")
    NAME_FIELD_NUMBER: _ClassVar[int]
    AUTHORITY_FIELD_NUMBER: _ClassVar[int]
    ANSWERS_FIELD_NUMBER: _ClassVar[int]
    BOUNDS_FIELD_NUMBER: _ClassVar[int]
    PROCESS_FIELD_NUMBER: _ClassVar[int]
    PROCESS_FOUND_FIELD_NUMBER: _ClassVar[int]
    READY_FIELD_NUMBER: _ClassVar[int]
    UNAVAILABLE_FIELD_NUMBER: _ClassVar[int]
    name: str
    authority: str
    answers: _containers.RepeatedScalarFieldContainer[str]
    bounds: _containers.RepeatedScalarFieldContainer[str]
    process: str
    process_found: str
    ready: bool
    unavailable: str
    def __init__(self, name: _Optional[str] = ..., authority: _Optional[str] = ..., answers: _Optional[_Iterable[str]] = ..., bounds: _Optional[_Iterable[str]] = ..., process: _Optional[str] = ..., process_found: _Optional[str] = ..., ready: _Optional[bool] = ..., unavailable: _Optional[str] = ...) -> None: ...

class ListEnginesResponse(_message.Message):
    __slots__ = ("engines",)
    ENGINES_FIELD_NUMBER: _ClassVar[int]
    engines: _containers.RepeatedCompositeFieldContainer[EngineInfo]
    def __init__(self, engines: _Optional[_Iterable[_Union[EngineInfo, _Mapping]]] = ...) -> None: ...

class ParseFileRequest(_message.Message):
    __slots__ = ("file_path", "content", "content_hash", "language", "strict_conformance")
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    CONTENT_HASH_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    STRICT_CONFORMANCE_FIELD_NUMBER: _ClassVar[int]
    file_path: str
    content: str
    content_hash: str
    language: str
    strict_conformance: bool
    def __init__(self, file_path: _Optional[str] = ..., content: _Optional[str] = ..., content_hash: _Optional[str] = ..., language: _Optional[str] = ..., strict_conformance: _Optional[bool] = ...) -> None: ...

class SourceDocument(_message.Message):
    __slots__ = ("file_path", "content", "language", "name")
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    file_path: str
    content: str
    language: str
    name: str
    def __init__(self, file_path: _Optional[str] = ..., content: _Optional[str] = ..., language: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class ParseSourcesRequest(_message.Message):
    __slots__ = ("documents", "strict_conformance")
    DOCUMENTS_FIELD_NUMBER: _ClassVar[int]
    STRICT_CONFORMANCE_FIELD_NUMBER: _ClassVar[int]
    documents: _containers.RepeatedCompositeFieldContainer[SourceDocument]
    strict_conformance: bool
    def __init__(self, documents: _Optional[_Iterable[_Union[SourceDocument, _Mapping]]] = ..., strict_conformance: _Optional[bool] = ...) -> None: ...

class ParseSourcesResponse(_message.Message):
    __slots__ = ("model_hash", "roots", "diagnostics", "error")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    ROOTS_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    roots: _containers.RepeatedCompositeFieldContainer[SymbolInfo]
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    error: str
    def __init__(self, model_hash: _Optional[str] = ..., roots: _Optional[_Iterable[_Union[SymbolInfo, _Mapping]]] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., error: _Optional[str] = ...) -> None: ...

class ParseFileResponse(_message.Message):
    __slots__ = ("model_hash", "root", "diagnostics", "error")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    ROOT_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    root: SymbolInfo
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    error: str
    def __init__(self, model_hash: _Optional[str] = ..., root: _Optional[_Union[SymbolInfo, _Mapping]] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., error: _Optional[str] = ...) -> None: ...

class GetSymbolRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ...) -> None: ...

class SymbolResponse(_message.Message):
    __slots__ = ("symbol", "error")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    symbol: SymbolInfo
    error: str
    def __init__(self, symbol: _Optional[_Union[SymbolInfo, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...

class DiagnosticsRequest(_message.Message):
    __slots__ = ("model_hash",)
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    def __init__(self, model_hash: _Optional[str] = ...) -> None: ...

class DiagnosticsResponse(_message.Message):
    __slots__ = ("diagnostics", "error")
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    error: str
    def __init__(self, diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., error: _Optional[str] = ...) -> None: ...

class EvaluateRequest(_message.Message):
    __slots__ = ("model_hash", "expression", "context_symbol_id", "subject_symbol_id")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    EXPRESSION_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    expression: str
    context_symbol_id: str
    subject_symbol_id: str
    def __init__(self, model_hash: _Optional[str] = ..., expression: _Optional[str] = ..., context_symbol_id: _Optional[str] = ..., subject_symbol_id: _Optional[str] = ...) -> None: ...

class EvaluateResponse(_message.Message):
    __slots__ = ("result", "error", "diagnostics")
    RESULT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    result: Value
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    def __init__(self, result: _Optional[_Union[Value, _Mapping]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ...) -> None: ...

class Instance(_message.Message):
    __slots__ = ("id", "type_symbol_id", "feature_values")
    class FeatureValuesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: FeatureValue
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[FeatureValue, _Mapping]] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    FEATURE_VALUES_FIELD_NUMBER: _ClassVar[int]
    id: int
    type_symbol_id: str
    feature_values: _containers.MessageMap[str, FeatureValue]
    def __init__(self, id: _Optional[int] = ..., type_symbol_id: _Optional[str] = ..., feature_values: _Optional[_Mapping[str, FeatureValue]] = ...) -> None: ...

class FeatureValue(_message.Message):
    __slots__ = ("feature_name", "value", "values", "materialized", "error")
    FEATURE_NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    MATERIALIZED_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    feature_name: str
    value: Value
    values: _containers.RepeatedCompositeFieldContainer[Value]
    materialized: bool
    error: str
    def __init__(self, feature_name: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ..., values: _Optional[_Iterable[_Union[Value, _Mapping]]] = ..., materialized: _Optional[bool] = ..., error: _Optional[str] = ...) -> None: ...

class InstantiateRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ...) -> None: ...

class InstantiateResponse(_message.Message):
    __slots__ = ("instance", "error", "diagnostics", "instances")
    INSTANCE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    instance: Instance
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    instances: _containers.RepeatedCompositeFieldContainer[Instance]
    def __init__(self, instance: _Optional[_Union[Instance, _Mapping]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., instances: _Optional[_Iterable[_Union[Instance, _Mapping]]] = ...) -> None: ...

class ExecuteActionRequest(_message.Message):
    __slots__ = ("model_hash", "action_symbol_id", "inputs", "schedule")
    class InputsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Value
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    ACTION_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    INPUTS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    action_symbol_id: str
    inputs: _containers.MessageMap[str, Value]
    schedule: str
    def __init__(self, model_hash: _Optional[str] = ..., action_symbol_id: _Optional[str] = ..., inputs: _Optional[_Mapping[str, Value]] = ..., schedule: _Optional[str] = ...) -> None: ...

class ExecuteActionResponse(_message.Message):
    __slots__ = ("outputs", "error", "diagnostics", "outcomes", "exploration", "final_time")
    class OutputsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Value
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    OUTCOMES_FIELD_NUMBER: _ClassVar[int]
    EXPLORATION_FIELD_NUMBER: _ClassVar[int]
    FINAL_TIME_FIELD_NUMBER: _ClassVar[int]
    outputs: _containers.MessageMap[str, Value]
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    outcomes: _containers.RepeatedCompositeFieldContainer[Outcome]
    exploration: ExplorationStatus
    final_time: float
    def __init__(self, outputs: _Optional[_Mapping[str, Value]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., outcomes: _Optional[_Iterable[_Union[Outcome, _Mapping]]] = ..., exploration: _Optional[_Union[ExplorationStatus, _Mapping]] = ..., final_time: _Optional[float] = ...) -> None: ...

class ExecuteStateRequest(_message.Message):
    __slots__ = ("model_hash", "state_machine_symbol_id", "events", "schedule")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    STATE_MACHINE_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    state_machine_symbol_id: str
    events: _containers.RepeatedScalarFieldContainer[str]
    schedule: str
    def __init__(self, model_hash: _Optional[str] = ..., state_machine_symbol_id: _Optional[str] = ..., events: _Optional[_Iterable[str]] = ..., schedule: _Optional[str] = ...) -> None: ...

class ExecuteStateResponse(_message.Message):
    __slots__ = ("states_visited", "final_context", "error", "diagnostics", "outcomes", "exploration", "final_time")
    class FinalContextEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Value
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...
    STATES_VISITED_FIELD_NUMBER: _ClassVar[int]
    FINAL_CONTEXT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    OUTCOMES_FIELD_NUMBER: _ClassVar[int]
    EXPLORATION_FIELD_NUMBER: _ClassVar[int]
    FINAL_TIME_FIELD_NUMBER: _ClassVar[int]
    states_visited: _containers.RepeatedScalarFieldContainer[str]
    final_context: _containers.MessageMap[str, Value]
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    outcomes: _containers.RepeatedCompositeFieldContainer[Outcome]
    exploration: ExplorationStatus
    final_time: float
    def __init__(self, states_visited: _Optional[_Iterable[str]] = ..., final_context: _Optional[_Mapping[str, Value]] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., outcomes: _Optional[_Iterable[_Union[Outcome, _Mapping]]] = ..., exploration: _Optional[_Union[ExplorationStatus, _Mapping]] = ..., final_time: _Optional[float] = ...) -> None: ...

class ConvertRequest(_message.Message):
    __slots__ = ("file_path", "content", "model_hash", "from_format", "to_format", "tolerate_syntax_errors")
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    FROM_FORMAT_FIELD_NUMBER: _ClassVar[int]
    TO_FORMAT_FIELD_NUMBER: _ClassVar[int]
    TOLERATE_SYNTAX_ERRORS_FIELD_NUMBER: _ClassVar[int]
    file_path: str
    content: str
    model_hash: str
    from_format: str
    to_format: str
    tolerate_syntax_errors: bool
    def __init__(self, file_path: _Optional[str] = ..., content: _Optional[str] = ..., model_hash: _Optional[str] = ..., from_format: _Optional[str] = ..., to_format: _Optional[str] = ..., tolerate_syntax_errors: _Optional[bool] = ...) -> None: ...

class ConvertResponse(_message.Message):
    __slots__ = ("content", "from_format", "to_format", "error", "diagnostics", "experimental", "experimental_notice")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    FROM_FORMAT_FIELD_NUMBER: _ClassVar[int]
    TO_FORMAT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    EXPERIMENTAL_FIELD_NUMBER: _ClassVar[int]
    EXPERIMENTAL_NOTICE_FIELD_NUMBER: _ClassVar[int]
    content: str
    from_format: str
    to_format: str
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    experimental: bool
    experimental_notice: str
    def __init__(self, content: _Optional[str] = ..., from_format: _Optional[str] = ..., to_format: _Optional[str] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., experimental: _Optional[bool] = ..., experimental_notice: _Optional[str] = ...) -> None: ...

class ApplyEditsRequest(_message.Message):
    __slots__ = ("model_hash", "operations")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    OPERATIONS_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    operations: _containers.RepeatedCompositeFieldContainer[EditOperation]
    def __init__(self, model_hash: _Optional[str] = ..., operations: _Optional[_Iterable[_Union[EditOperation, _Mapping]]] = ...) -> None: ...

class EditOperation(_message.Message):
    __slots__ = ("set_value", "rename", "add_member", "delete")
    SET_VALUE_FIELD_NUMBER: _ClassVar[int]
    RENAME_FIELD_NUMBER: _ClassVar[int]
    ADD_MEMBER_FIELD_NUMBER: _ClassVar[int]
    DELETE_FIELD_NUMBER: _ClassVar[int]
    set_value: SetValueEdit
    rename: RenameEdit
    add_member: AddMemberEdit
    delete: DeleteEdit
    def __init__(self, set_value: _Optional[_Union[SetValueEdit, _Mapping]] = ..., rename: _Optional[_Union[RenameEdit, _Mapping]] = ..., add_member: _Optional[_Union[AddMemberEdit, _Mapping]] = ..., delete: _Optional[_Union[DeleteEdit, _Mapping]] = ...) -> None: ...

class AddMemberEdit(_message.Message):
    __slots__ = ("owner", "kind", "name", "type", "multiplicity", "value", "specializes")
    OWNER_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    MULTIPLICITY_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    SPECIALIZES_FIELD_NUMBER: _ClassVar[int]
    owner: str
    kind: str
    name: str
    type: str
    multiplicity: str
    value: str
    specializes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, owner: _Optional[str] = ..., kind: _Optional[str] = ..., name: _Optional[str] = ..., type: _Optional[str] = ..., multiplicity: _Optional[str] = ..., value: _Optional[str] = ..., specializes: _Optional[_Iterable[str]] = ...) -> None: ...

class DeleteEdit(_message.Message):
    __slots__ = ("target", "cascade")
    TARGET_FIELD_NUMBER: _ClassVar[int]
    CASCADE_FIELD_NUMBER: _ClassVar[int]
    target: str
    cascade: bool
    def __init__(self, target: _Optional[str] = ..., cascade: _Optional[bool] = ...) -> None: ...

class SetValueEdit(_message.Message):
    __slots__ = ("target", "value")
    TARGET_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    target: str
    value: str
    def __init__(self, target: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

class RenameEdit(_message.Message):
    __slots__ = ("target", "new_name")
    TARGET_FIELD_NUMBER: _ClassVar[int]
    NEW_NAME_FIELD_NUMBER: _ClassVar[int]
    target: str
    new_name: str
    def __init__(self, target: _Optional[str] = ..., new_name: _Optional[str] = ...) -> None: ...

class ApplyEditsResponse(_message.Message):
    __slots__ = ("content", "applied", "error", "failure", "diagnostics", "referring_elements")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    APPLIED_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    REFERRING_ELEMENTS_FIELD_NUMBER: _ClassVar[int]
    content: str
    applied: _containers.RepeatedCompositeFieldContainer[AppliedEdit]
    error: str
    failure: EditFailure
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    referring_elements: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, content: _Optional[str] = ..., applied: _Optional[_Iterable[_Union[AppliedEdit, _Mapping]]] = ..., error: _Optional[str] = ..., failure: _Optional[_Union[EditFailure, str]] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., referring_elements: _Optional[_Iterable[str]] = ...) -> None: ...

class AppliedEdit(_message.Message):
    __slots__ = ("operation_index", "target", "offset", "length", "old_text", "new_text")
    OPERATION_INDEX_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    LENGTH_FIELD_NUMBER: _ClassVar[int]
    OLD_TEXT_FIELD_NUMBER: _ClassVar[int]
    NEW_TEXT_FIELD_NUMBER: _ClassVar[int]
    operation_index: int
    target: str
    offset: int
    length: int
    old_text: str
    new_text: str
    def __init__(self, operation_index: _Optional[int] = ..., target: _Optional[str] = ..., offset: _Optional[int] = ..., length: _Optional[int] = ..., old_text: _Optional[str] = ..., new_text: _Optional[str] = ...) -> None: ...

class SymbolInfo(_message.Message):
    __slots__ = ("id", "name", "kind", "metadata", "child_ids", "attributes", "type_info", "multiplicity", "specializations", "withheld_library_attributes")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    CHILD_IDS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    TYPE_INFO_FIELD_NUMBER: _ClassVar[int]
    MULTIPLICITY_FIELD_NUMBER: _ClassVar[int]
    SPECIALIZATIONS_FIELD_NUMBER: _ClassVar[int]
    WITHHELD_LIBRARY_ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    kind: str
    metadata: _containers.ScalarMap[str, str]
    child_ids: _containers.RepeatedScalarFieldContainer[str]
    attributes: _containers.RepeatedCompositeFieldContainer[AttributeInfo]
    type_info: TypeInfo
    multiplicity: MultiplicityInfo
    specializations: _containers.RepeatedCompositeFieldContainer[Specialization]
    withheld_library_attributes: int
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., kind: _Optional[str] = ..., metadata: _Optional[_Mapping[str, str]] = ..., child_ids: _Optional[_Iterable[str]] = ..., attributes: _Optional[_Iterable[_Union[AttributeInfo, _Mapping]]] = ..., type_info: _Optional[_Union[TypeInfo, _Mapping]] = ..., multiplicity: _Optional[_Union[MultiplicityInfo, _Mapping]] = ..., specializations: _Optional[_Iterable[_Union[Specialization, _Mapping]]] = ..., withheld_library_attributes: _Optional[int] = ...) -> None: ...

class Specialization(_message.Message):
    __slots__ = ("kind", "declared", "target_id", "target_kind")
    KIND_FIELD_NUMBER: _ClassVar[int]
    DECLARED_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_KIND_FIELD_NUMBER: _ClassVar[int]
    kind: str
    declared: str
    target_id: str
    target_kind: str
    def __init__(self, kind: _Optional[str] = ..., declared: _Optional[str] = ..., target_id: _Optional[str] = ..., target_kind: _Optional[str] = ...) -> None: ...

class TypeInfo(_message.Message):
    __slots__ = ("declared", "resolved_id", "resolved_kind", "primitive", "primitive_source", "quantity", "unit")
    DECLARED_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_ID_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_KIND_FIELD_NUMBER: _ClassVar[int]
    PRIMITIVE_FIELD_NUMBER: _ClassVar[int]
    PRIMITIVE_SOURCE_FIELD_NUMBER: _ClassVar[int]
    QUANTITY_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    declared: str
    resolved_id: str
    resolved_kind: str
    primitive: str
    primitive_source: str
    quantity: bool
    unit: str
    def __init__(self, declared: _Optional[str] = ..., resolved_id: _Optional[str] = ..., resolved_kind: _Optional[str] = ..., primitive: _Optional[str] = ..., primitive_source: _Optional[str] = ..., quantity: _Optional[bool] = ..., unit: _Optional[str] = ...) -> None: ...

class MultiplicityInfo(_message.Message):
    __slots__ = ("lower", "upper")
    LOWER_FIELD_NUMBER: _ClassVar[int]
    UPPER_FIELD_NUMBER: _ClassVar[int]
    lower: str
    upper: str
    def __init__(self, lower: _Optional[str] = ..., upper: _Optional[str] = ...) -> None: ...

class AttributeInfo(_message.Message):
    __slots__ = ("name", "type", "value", "unit")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    name: str
    type: str
    value: Value
    unit: str
    def __init__(self, name: _Optional[str] = ..., type: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ..., unit: _Optional[str] = ...) -> None: ...

class Value(_message.Message):
    __slots__ = ("int_value", "real_value", "bool_value", "string_value", "instance_id", "sequence", "null", "quantity", "enum_literal", "unset", "complex", "array", "vector", "vector_quantity", "measurement_ref", "infinity", "function", "set", "tensor_quantity", "metaobject")
    INT_VALUE_FIELD_NUMBER: _ClassVar[int]
    REAL_VALUE_FIELD_NUMBER: _ClassVar[int]
    BOOL_VALUE_FIELD_NUMBER: _ClassVar[int]
    STRING_VALUE_FIELD_NUMBER: _ClassVar[int]
    INSTANCE_ID_FIELD_NUMBER: _ClassVar[int]
    SEQUENCE_FIELD_NUMBER: _ClassVar[int]
    NULL_FIELD_NUMBER: _ClassVar[int]
    QUANTITY_FIELD_NUMBER: _ClassVar[int]
    ENUM_LITERAL_FIELD_NUMBER: _ClassVar[int]
    UNSET_FIELD_NUMBER: _ClassVar[int]
    COMPLEX_FIELD_NUMBER: _ClassVar[int]
    ARRAY_FIELD_NUMBER: _ClassVar[int]
    VECTOR_FIELD_NUMBER: _ClassVar[int]
    VECTOR_QUANTITY_FIELD_NUMBER: _ClassVar[int]
    MEASUREMENT_REF_FIELD_NUMBER: _ClassVar[int]
    INFINITY_FIELD_NUMBER: _ClassVar[int]
    FUNCTION_FIELD_NUMBER: _ClassVar[int]
    SET_FIELD_NUMBER: _ClassVar[int]
    TENSOR_QUANTITY_FIELD_NUMBER: _ClassVar[int]
    METAOBJECT_FIELD_NUMBER: _ClassVar[int]
    int_value: int
    real_value: float
    bool_value: bool
    string_value: str
    instance_id: int
    sequence: ValueSequence
    null: str
    quantity: Quantity
    enum_literal: EnumLiteral
    unset: bool
    complex: Complex
    array: Array
    vector: Vector
    vector_quantity: VectorQuantity
    measurement_ref: MeasurementRef
    infinity: bool
    function: Function
    set: ValueSet
    tensor_quantity: TensorQuantity
    metaobject: Metaobject
    def __init__(self, int_value: _Optional[int] = ..., real_value: _Optional[float] = ..., bool_value: _Optional[bool] = ..., string_value: _Optional[str] = ..., instance_id: _Optional[int] = ..., sequence: _Optional[_Union[ValueSequence, _Mapping]] = ..., null: _Optional[str] = ..., quantity: _Optional[_Union[Quantity, _Mapping]] = ..., enum_literal: _Optional[_Union[EnumLiteral, _Mapping]] = ..., unset: _Optional[bool] = ..., complex: _Optional[_Union[Complex, _Mapping]] = ..., array: _Optional[_Union[Array, _Mapping]] = ..., vector: _Optional[_Union[Vector, _Mapping]] = ..., vector_quantity: _Optional[_Union[VectorQuantity, _Mapping]] = ..., measurement_ref: _Optional[_Union[MeasurementRef, _Mapping]] = ..., infinity: _Optional[bool] = ..., function: _Optional[_Union[Function, _Mapping]] = ..., set: _Optional[_Union[ValueSet, _Mapping]] = ..., tensor_quantity: _Optional[_Union[TensorQuantity, _Mapping]] = ..., metaobject: _Optional[_Union[Metaobject, _Mapping]] = ...) -> None: ...

class Metaobject(_message.Message):
    __slots__ = ("element_id", "metaclass_id")
    ELEMENT_ID_FIELD_NUMBER: _ClassVar[int]
    METACLASS_ID_FIELD_NUMBER: _ClassVar[int]
    element_id: str
    metaclass_id: str
    def __init__(self, element_id: _Optional[str] = ..., metaclass_id: _Optional[str] = ...) -> None: ...

class Function(_message.Message):
    __slots__ = ("calc_id", "self_id")
    CALC_ID_FIELD_NUMBER: _ClassVar[int]
    SELF_ID_FIELD_NUMBER: _ClassVar[int]
    calc_id: str
    self_id: int
    def __init__(self, calc_id: _Optional[str] = ..., self_id: _Optional[int] = ...) -> None: ...

class ValueSet(_message.Message):
    __slots__ = ("elements",)
    ELEMENTS_FIELD_NUMBER: _ClassVar[int]
    elements: _containers.RepeatedCompositeFieldContainer[Value]
    def __init__(self, elements: _Optional[_Iterable[_Union[Value, _Mapping]]] = ...) -> None: ...

class TensorQuantity(_message.Message):
    __slots__ = ("dimensions", "components")
    DIMENSIONS_FIELD_NUMBER: _ClassVar[int]
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    dimensions: _containers.RepeatedScalarFieldContainer[int]
    components: _containers.RepeatedCompositeFieldContainer[Quantity]
    def __init__(self, dimensions: _Optional[_Iterable[int]] = ..., components: _Optional[_Iterable[_Union[Quantity, _Mapping]]] = ...) -> None: ...

class Array(_message.Message):
    __slots__ = ("dimensions", "elements")
    DIMENSIONS_FIELD_NUMBER: _ClassVar[int]
    ELEMENTS_FIELD_NUMBER: _ClassVar[int]
    dimensions: _containers.RepeatedScalarFieldContainer[int]
    elements: _containers.RepeatedCompositeFieldContainer[Value]
    def __init__(self, dimensions: _Optional[_Iterable[int]] = ..., elements: _Optional[_Iterable[_Union[Value, _Mapping]]] = ...) -> None: ...

class Vector(_message.Message):
    __slots__ = ("components",)
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    components: _containers.RepeatedCompositeFieldContainer[Value]
    def __init__(self, components: _Optional[_Iterable[_Union[Value, _Mapping]]] = ...) -> None: ...

class VectorQuantity(_message.Message):
    __slots__ = ("components",)
    COMPONENTS_FIELD_NUMBER: _ClassVar[int]
    components: _containers.RepeatedCompositeFieldContainer[Quantity]
    def __init__(self, components: _Optional[_Iterable[_Union[Quantity, _Mapping]]] = ...) -> None: ...

class Complex(_message.Message):
    __slots__ = ("real", "imaginary")
    REAL_FIELD_NUMBER: _ClassVar[int]
    IMAGINARY_FIELD_NUMBER: _ClassVar[int]
    real: float
    imaginary: float
    def __init__(self, real: _Optional[float] = ..., imaginary: _Optional[float] = ...) -> None: ...

class EnumLiteral(_message.Message):
    __slots__ = ("literal_id", "enumeration_id", "name")
    LITERAL_ID_FIELD_NUMBER: _ClassVar[int]
    ENUMERATION_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    literal_id: str
    enumeration_id: str
    name: str
    def __init__(self, literal_id: _Optional[str] = ..., enumeration_id: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class ValueSequence(_message.Message):
    __slots__ = ("elements",)
    ELEMENTS_FIELD_NUMBER: _ClassVar[int]
    elements: _containers.RepeatedCompositeFieldContainer[Value]
    def __init__(self, elements: _Optional[_Iterable[_Union[Value, _Mapping]]] = ...) -> None: ...

class Quantity(_message.Message):
    __slots__ = ("int_magnitude", "real_magnitude", "unit", "unit_term")
    INT_MAGNITUDE_FIELD_NUMBER: _ClassVar[int]
    REAL_MAGNITUDE_FIELD_NUMBER: _ClassVar[int]
    UNIT_FIELD_NUMBER: _ClassVar[int]
    UNIT_TERM_FIELD_NUMBER: _ClassVar[int]
    int_magnitude: int
    real_magnitude: float
    unit: str
    unit_term: UnitTerm
    def __init__(self, int_magnitude: _Optional[int] = ..., real_magnitude: _Optional[float] = ..., unit: _Optional[str] = ..., unit_term: _Optional[_Union[UnitTerm, _Mapping]] = ...) -> None: ...

class MeasurementRef(_message.Message):
    __slots__ = ("unit", "unit_term", "unit_id")
    UNIT_FIELD_NUMBER: _ClassVar[int]
    UNIT_TERM_FIELD_NUMBER: _ClassVar[int]
    UNIT_ID_FIELD_NUMBER: _ClassVar[int]
    unit: str
    unit_term: UnitTerm
    unit_id: str
    def __init__(self, unit: _Optional[str] = ..., unit_term: _Optional[_Union[UnitTerm, _Mapping]] = ..., unit_id: _Optional[str] = ...) -> None: ...

class UnitTerm(_message.Message):
    __slots__ = ("scale_num", "scale_den", "factors")
    SCALE_NUM_FIELD_NUMBER: _ClassVar[int]
    SCALE_DEN_FIELD_NUMBER: _ClassVar[int]
    FACTORS_FIELD_NUMBER: _ClassVar[int]
    scale_num: float
    scale_den: float
    factors: _containers.RepeatedCompositeFieldContainer[UnitFactor]
    def __init__(self, scale_num: _Optional[float] = ..., scale_den: _Optional[float] = ..., factors: _Optional[_Iterable[_Union[UnitFactor, _Mapping]]] = ...) -> None: ...

class UnitFactor(_message.Message):
    __slots__ = ("unit_id", "exponent")
    UNIT_ID_FIELD_NUMBER: _ClassVar[int]
    EXPONENT_FIELD_NUMBER: _ClassVar[int]
    unit_id: str
    exponent: float
    def __init__(self, unit_id: _Optional[str] = ..., exponent: _Optional[float] = ...) -> None: ...

class Diagnostic(_message.Message):
    __slots__ = ("severity", "message", "span", "code")
    SEVERITY_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    SPAN_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    severity: str
    message: str
    span: Span
    code: str
    def __init__(self, severity: _Optional[str] = ..., message: _Optional[str] = ..., span: _Optional[_Union[Span, _Mapping]] = ..., code: _Optional[str] = ...) -> None: ...

class Span(_message.Message):
    __slots__ = ("file", "start_line", "start_col", "end_line", "end_col")
    FILE_FIELD_NUMBER: _ClassVar[int]
    START_LINE_FIELD_NUMBER: _ClassVar[int]
    START_COL_FIELD_NUMBER: _ClassVar[int]
    END_LINE_FIELD_NUMBER: _ClassVar[int]
    END_COL_FIELD_NUMBER: _ClassVar[int]
    file: str
    start_line: int
    start_col: int
    end_line: int
    end_col: int
    def __init__(self, file: _Optional[str] = ..., start_line: _Optional[int] = ..., start_col: _Optional[int] = ..., end_line: _Optional[int] = ..., end_col: _Optional[int] = ...) -> None: ...

class ServerInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ServerInfoResponse(_message.Message):
    __slots__ = ("version", "capabilities")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    CAPABILITIES_FIELD_NUMBER: _ClassVar[int]
    version: str
    capabilities: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, version: _Optional[str] = ..., capabilities: _Optional[_Iterable[str]] = ...) -> None: ...

class QueryRequest(_message.Message):
    __slots__ = ("model_hash", "query", "oslc_query")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    OSLC_QUERY_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    query: Query
    oslc_query: str
    def __init__(self, model_hash: _Optional[str] = ..., query: _Optional[_Union[Query, _Mapping]] = ..., oslc_query: _Optional[str] = ...) -> None: ...

class QueryResponse(_message.Message):
    __slots__ = ("elements",)
    ELEMENTS_FIELD_NUMBER: _ClassVar[int]
    elements: _containers.RepeatedCompositeFieldContainer[QueryResultElement]
    def __init__(self, elements: _Optional[_Iterable[_Union[QueryResultElement, _Mapping]]] = ...) -> None: ...

class Query(_message.Message):
    __slots__ = ("scope", "select", "where")
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    SELECT_FIELD_NUMBER: _ClassVar[int]
    WHERE_FIELD_NUMBER: _ClassVar[int]
    scope: _containers.RepeatedScalarFieldContainer[str]
    select: _containers.RepeatedScalarFieldContainer[str]
    where: Constraint
    def __init__(self, scope: _Optional[_Iterable[str]] = ..., select: _Optional[_Iterable[str]] = ..., where: _Optional[_Union[Constraint, _Mapping]] = ...) -> None: ...

class Constraint(_message.Message):
    __slots__ = ("primitive", "composite")
    PRIMITIVE_FIELD_NUMBER: _ClassVar[int]
    COMPOSITE_FIELD_NUMBER: _ClassVar[int]
    primitive: PrimitiveConstraint
    composite: CompositeConstraint
    def __init__(self, primitive: _Optional[_Union[PrimitiveConstraint, _Mapping]] = ..., composite: _Optional[_Union[CompositeConstraint, _Mapping]] = ...) -> None: ...

class PrimitiveConstraint(_message.Message):
    __slots__ = ("inverse", "property", "operator", "value")
    INVERSE_FIELD_NUMBER: _ClassVar[int]
    PROPERTY_FIELD_NUMBER: _ClassVar[int]
    OPERATOR_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    inverse: bool
    property: str
    operator: PrimitiveOperator
    value: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, inverse: _Optional[bool] = ..., property: _Optional[str] = ..., operator: _Optional[_Union[PrimitiveOperator, str]] = ..., value: _Optional[_Iterable[str]] = ...) -> None: ...

class CompositeConstraint(_message.Message):
    __slots__ = ("operator", "constraint")
    OPERATOR_FIELD_NUMBER: _ClassVar[int]
    CONSTRAINT_FIELD_NUMBER: _ClassVar[int]
    operator: CompositeOperator
    constraint: _containers.RepeatedCompositeFieldContainer[Constraint]
    def __init__(self, operator: _Optional[_Union[CompositeOperator, str]] = ..., constraint: _Optional[_Iterable[_Union[Constraint, _Mapping]]] = ...) -> None: ...

class QueryResultElement(_message.Message):
    __slots__ = ("id", "type", "properties")
    class PropertiesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PROPERTIES_FIELD_NUMBER: _ClassVar[int]
    id: str
    type: str
    properties: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., type: _Optional[str] = ..., properties: _Optional[_Mapping[str, str]] = ...) -> None: ...

class SweepRange(_message.Message):
    __slots__ = ("parameter", "start", "end", "step")
    PARAMETER_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    STEP_FIELD_NUMBER: _ClassVar[int]
    parameter: str
    start: Value
    end: Value
    step: Value
    def __init__(self, parameter: _Optional[str] = ..., start: _Optional[_Union[Value, _Mapping]] = ..., end: _Optional[_Union[Value, _Mapping]] = ..., step: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...

class RunSweepRequest(_message.Message):
    __slots__ = ("model_hash", "symbol_id", "subject_symbol_id", "arguments", "named_arguments", "ranges", "samples", "seed", "engine")
    class NamedArgumentsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Value
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Value, _Mapping]] = ...) -> None: ...
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_SYMBOL_ID_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    NAMED_ARGUMENTS_FIELD_NUMBER: _ClassVar[int]
    RANGES_FIELD_NUMBER: _ClassVar[int]
    SAMPLES_FIELD_NUMBER: _ClassVar[int]
    SEED_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    symbol_id: str
    subject_symbol_id: str
    arguments: _containers.RepeatedCompositeFieldContainer[Value]
    named_arguments: _containers.MessageMap[str, Value]
    ranges: _containers.RepeatedCompositeFieldContainer[SweepRange]
    samples: int
    seed: int
    engine: str
    def __init__(self, model_hash: _Optional[str] = ..., symbol_id: _Optional[str] = ..., subject_symbol_id: _Optional[str] = ..., arguments: _Optional[_Iterable[_Union[Value, _Mapping]]] = ..., named_arguments: _Optional[_Mapping[str, Value]] = ..., ranges: _Optional[_Iterable[_Union[SweepRange, _Mapping]]] = ..., samples: _Optional[int] = ..., seed: _Optional[int] = ..., engine: _Optional[str] = ...) -> None: ...

class SweepRow(_message.Message):
    __slots__ = ("inputs", "outputs", "verdicts", "elapsed_micros", "error", "failure_reason", "evaluations")
    INPUTS_FIELD_NUMBER: _ClassVar[int]
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    VERDICTS_FIELD_NUMBER: _ClassVar[int]
    ELAPSED_MICROS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    FAILURE_REASON_FIELD_NUMBER: _ClassVar[int]
    EVALUATIONS_FIELD_NUMBER: _ClassVar[int]
    inputs: _containers.RepeatedCompositeFieldContainer[CalcOutput]
    outputs: _containers.RepeatedCompositeFieldContainer[CalcOutput]
    verdicts: _containers.RepeatedCompositeFieldContainer[Verdict]
    elapsed_micros: int
    error: str
    failure_reason: FailureReason
    evaluations: _containers.RepeatedCompositeFieldContainer[CaseEvaluation]
    def __init__(self, inputs: _Optional[_Iterable[_Union[CalcOutput, _Mapping]]] = ..., outputs: _Optional[_Iterable[_Union[CalcOutput, _Mapping]]] = ..., verdicts: _Optional[_Iterable[_Union[Verdict, _Mapping]]] = ..., elapsed_micros: _Optional[int] = ..., error: _Optional[str] = ..., failure_reason: _Optional[_Union[FailureReason, str]] = ..., evaluations: _Optional[_Iterable[_Union[CaseEvaluation, _Mapping]]] = ...) -> None: ...

class RunSweepResponse(_message.Message):
    __slots__ = ("rows", "parameters", "sampled", "seed", "error", "diagnostics", "failure_reason", "instances", "engine", "strength", "bounds")
    ROWS_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    SAMPLED_FIELD_NUMBER: _ClassVar[int]
    SEED_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    DIAGNOSTICS_FIELD_NUMBER: _ClassVar[int]
    FAILURE_REASON_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    STRENGTH_FIELD_NUMBER: _ClassVar[int]
    BOUNDS_FIELD_NUMBER: _ClassVar[int]
    rows: _containers.RepeatedCompositeFieldContainer[SweepRow]
    parameters: _containers.RepeatedScalarFieldContainer[str]
    sampled: bool
    seed: int
    error: str
    diagnostics: _containers.RepeatedCompositeFieldContainer[Diagnostic]
    failure_reason: FailureReason
    instances: _containers.RepeatedCompositeFieldContainer[Instance]
    engine: str
    strength: str
    bounds: _containers.RepeatedCompositeFieldContainer[Bound]
    def __init__(self, rows: _Optional[_Iterable[_Union[SweepRow, _Mapping]]] = ..., parameters: _Optional[_Iterable[str]] = ..., sampled: _Optional[bool] = ..., seed: _Optional[int] = ..., error: _Optional[str] = ..., diagnostics: _Optional[_Iterable[_Union[Diagnostic, _Mapping]]] = ..., failure_reason: _Optional[_Union[FailureReason, str]] = ..., instances: _Optional[_Iterable[_Union[Instance, _Mapping]]] = ..., engine: _Optional[str] = ..., strength: _Optional[str] = ..., bounds: _Optional[_Iterable[_Union[Bound, _Mapping]]] = ...) -> None: ...

class RunDocumentQueryRequest(_message.Message):
    __slots__ = ("model_hash", "query_id", "bindings")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    QUERY_ID_FIELD_NUMBER: _ClassVar[int]
    BINDINGS_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    query_id: str
    bindings: _containers.RepeatedCompositeFieldContainer[DocumentQueryBinding]
    def __init__(self, model_hash: _Optional[str] = ..., query_id: _Optional[str] = ..., bindings: _Optional[_Iterable[_Union[DocumentQueryBinding, _Mapping]]] = ...) -> None: ...

class DocumentQueryBinding(_message.Message):
    __slots__ = ("parameter", "values")
    PARAMETER_FIELD_NUMBER: _ClassVar[int]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    parameter: str
    values: _containers.RepeatedCompositeFieldContainer[DocumentValue]
    def __init__(self, parameter: _Optional[str] = ..., values: _Optional[_Iterable[_Union[DocumentValue, _Mapping]]] = ...) -> None: ...

class DocumentValue(_message.Message):
    __slots__ = ("element_id", "string_value", "int_value", "real_value", "bool_value", "infinity", "quantity", "element_type")
    ELEMENT_ID_FIELD_NUMBER: _ClassVar[int]
    STRING_VALUE_FIELD_NUMBER: _ClassVar[int]
    INT_VALUE_FIELD_NUMBER: _ClassVar[int]
    REAL_VALUE_FIELD_NUMBER: _ClassVar[int]
    BOOL_VALUE_FIELD_NUMBER: _ClassVar[int]
    INFINITY_FIELD_NUMBER: _ClassVar[int]
    QUANTITY_FIELD_NUMBER: _ClassVar[int]
    ELEMENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    element_id: str
    string_value: str
    int_value: int
    real_value: float
    bool_value: bool
    infinity: bool
    quantity: Quantity
    element_type: str
    def __init__(self, element_id: _Optional[str] = ..., string_value: _Optional[str] = ..., int_value: _Optional[int] = ..., real_value: _Optional[float] = ..., bool_value: _Optional[bool] = ..., infinity: _Optional[bool] = ..., quantity: _Optional[_Union[Quantity, _Mapping]] = ..., element_type: _Optional[str] = ...) -> None: ...

class DocumentQueryColumn(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DocumentQueryCell(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedCompositeFieldContainer[DocumentValue]
    def __init__(self, values: _Optional[_Iterable[_Union[DocumentValue, _Mapping]]] = ...) -> None: ...

class DocumentQueryRow(_message.Message):
    __slots__ = ("element", "cells")
    ELEMENT_FIELD_NUMBER: _ClassVar[int]
    CELLS_FIELD_NUMBER: _ClassVar[int]
    element: DocumentValue
    cells: _containers.RepeatedCompositeFieldContainer[DocumentQueryCell]
    def __init__(self, element: _Optional[_Union[DocumentValue, _Mapping]] = ..., cells: _Optional[_Iterable[_Union[DocumentQueryCell, _Mapping]]] = ...) -> None: ...

class RunDocumentQueryResponse(_message.Message):
    __slots__ = ("columns", "rows")
    COLUMNS_FIELD_NUMBER: _ClassVar[int]
    ROWS_FIELD_NUMBER: _ClassVar[int]
    columns: _containers.RepeatedCompositeFieldContainer[DocumentQueryColumn]
    rows: _containers.RepeatedCompositeFieldContainer[DocumentQueryRow]
    def __init__(self, columns: _Optional[_Iterable[_Union[DocumentQueryColumn, _Mapping]]] = ..., rows: _Optional[_Iterable[_Union[DocumentQueryRow, _Mapping]]] = ...) -> None: ...

class RenderDocumentRequest(_message.Message):
    __slots__ = ("model_hash", "document_id")
    MODEL_HASH_FIELD_NUMBER: _ClassVar[int]
    DOCUMENT_ID_FIELD_NUMBER: _ClassVar[int]
    model_hash: str
    document_id: str
    def __init__(self, model_hash: _Optional[str] = ..., document_id: _Optional[str] = ...) -> None: ...

class RenderDocumentResponse(_message.Message):
    __slots__ = ("markdown",)
    MARKDOWN_FIELD_NUMBER: _ClassVar[int]
    markdown: str
    def __init__(self, markdown: _Optional[str] = ...) -> None: ...
