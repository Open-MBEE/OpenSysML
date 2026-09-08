// The isomorphic core: everything that does not need a process to spawn.

export { Connection } from "./connection.js";
export type {
  ConnectionBackend,
  Encoding,
  ResponseTap,
  TransportOptions,
} from "./connection.js";
export { Instance, InstanceTree, Model, ModelSymbol } from "./model.js";
export type {
  AttributeFacts,
  EvalOptions,
  FeatureValue,
  ParseOptions,
  SpecializationFacts,
  TypeFacts,
} from "./model.js";
export {
  CAPABILITY_APPLY_EDITS,
  CAPABILITY_CASE_EVALUATIONS,
  CAPABILITY_COMPLEX_VALUES,
  CAPABILITY_CONVERT,
  CAPABILITY_DIAGNOSTIC_CODES,
  CAPABILITY_ENUM_VALUES,
  CAPABILITY_EVALUATE_SUBJECT,
  CAPABILITY_FEATURE_VALUES,
  CAPABILITY_FUNCTION_VALUES,
  CAPABILITY_INFINITY_VALUE,
  CAPABILITY_INLINE_LANGUAGE,
  CAPABILITY_MEASUREMENT_REFS,
  CAPABILITY_QUERY,
  CAPABILITY_SCHEDULE,
  CAPABILITY_SET_VALUES,
  CAPABILITY_STRICT_CONFORMANCE,
  CAPABILITY_STRUCTURED_VALUES,
  CAPABILITY_VERIFICATION_VERDICTS,
  CAPABILITY_SYMBOL_ATTRIBUTES,
  CAPABILITY_TENSOR_VALUES,
  CAPABILITY_TYPE_FACTS,
  CAPABILITY_UNSET_VALUE,
  CAPABILITY_VERIFICATION,
  MissingCapabilityError,
  ServerInfo,
  requireCapability,
  upgradeRemedy,
} from "./capabilities.js";
export {
  ChecksumMismatchError,
  ClosedConnectionError,
  DownloadError,
  EvaluationError,
  InvalidRequestError,
  MalformedValueError,
  ManifestSignatureError,
  ModelFileNotFoundError,
  ModelNotFoundError,
  OpenSysMLError,
  ParseError,
  ServiceError,
  ServiceStartError,
  ServiceTimeoutError,
  SymbolNotFoundError,
  UnpinnedReleaseError,
  UnsignedReleaseError,
  UnsupportedOperationError,
} from "./errors.js";
export type { FailureCause, ModelDiagnostic } from "./errors.js";
export { fromHandshakeError, fromRpcError, statusName } from "./status.js";
export type { NotFoundSubject } from "./status.js";
export {
  decodeValue,
  decodeVerdict,
  encodeValue,
  formatValue,
  valuesEqual,
} from "./values.js";
export type {
  ArrayValue,
  ComplexValue,
  EnumValue,
  FunctionValue,
  Magnitude,
  MeasurementRefValue,
  QuantityValue,
  SysMLValue,
  SysMLVerdict,
  TensorQuantityValue,
  UnitFactor,
  UnitFactorization,
  VerdictSubject,
} from "./values.js";
export { baseUrl } from "./transport.js";
export { SysMLService } from "../generated/sysml_pb.js";
