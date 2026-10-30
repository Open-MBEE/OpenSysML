A submission's parse trees are let go once its files are accepted, instead of being held through
the analysis for the root namespaces an unresolved reference would be looked up by (#1441); a
reference left unresolved parses the text again. Validating a generated 12 086-element model
peaks at 164 MiB of resident memory instead of 184, beside 0.11.0's 164 on the same toolchain
(152 against 147 under `GOGC=25`), with the load's time and allocations unchanged.
