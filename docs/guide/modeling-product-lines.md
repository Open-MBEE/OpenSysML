# Modeling product lines and variability

A product line — a vehicle sold with two engines and two transmissions, a
spacecraft bus flown with or without a sunroof-sized radiator panel — is one
design with choices left open, and a product is that design with every choice
made. SysML v2 writes the open design as a *variation* whose *variants* are the
choices, ties the choices together with assertions, and writes a product as a
specialization that picks one variant per choice. This chapter builds the
vehicle family of the OMG training corpus from its superset model to its solved
configurations, and shows what the validator, the REPL, the solver and the
renderer each say about it. Every command and every output below was run on the
develop build.

The worked model is the three files of
`examples/sysml-v2-training/36. Variability/`, fetched with
`./scripts/download-training-examples.sh`. Three larger models are referenced
where they add something the small one lacks; they arrive with
`./scripts/download-pilot-corpora.sh`:

- `examples/pilot-corpora/sysml-examples/Variability Examples/VehicleVariabilityModel.sysml`,
  the OMG example the training files are cut from, with an optional sunroof and
  variation points inside actions;
- `examples/pilot-corpora/sysml-validation/07-Variant Configuration/7a-Variant Configuration - General Concept.sysml`,
  nested variation points and two configurations;
- `examples/pilot-corpora/sysml-validation/07-Variant Configuration/7b-Variant Configurations.sysml`,
  a family whose assertions tie an engine, a transmission and a requirement
  choice together, with a nested variation on the rear axle.

The vocabulary is that of SysML v2.1 Beta 2 §7.6.7 "Variations and Variants"
and of Annex A.12, the variability example; `./scripts/download-spec-examples.sh`
puts the text at `build/spec-docs/sysml.txt`. Build the tools once with
`make build`; `%configure` needs an SMT solver, see
[Solving the family](#solving-the-family).

## The superset model

## What the validator enforces

## Configurations

## Solving the family

## Seeing variability

## What the specification says that tools disagree on
