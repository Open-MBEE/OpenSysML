// Package fmi reads Functional Mock-up Units: the modelDescription.xml of an .fmu
// archive as a Description, and the `tool:fmi` analysis engine that computes a
// ToolExecution-annotated calc or action by handing the FMU to the runner
// OPENSYSML_FMI_RUNNER names. The package never runs the FMU's native code
// itself; the runner is the operator's grant of that execution.
package fmi
