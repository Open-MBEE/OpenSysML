# NASA-HDBK-1009A demo

[`hdbk1009-demo.sysml`](hdbk1009-demo.sysml) works a small fictional project — the *Dust Lidar
Pathfinder*, a lunar-surface dust lidar on a lander — down the metamodel of NASA-HDBK-1009A using
the bundled [`HDBK1009`](../../docs/project/hdbk1009-library.md) library. Each element carries the
Handbook's keyword, which both types it and adds it to the Handbook's base usage, and each arrow
of Figures 7-1 and 7-2 is the native SysML v2 relationship:

```sysml
#need requirement <'n-1'> characterizeDust;
#goal requirement <'g-1'> measureDustProfile;
#derivation connection { end #original ::> characterizeDust; end #derive ::> measureDustProfile; }
dependency from measureDustProfile to dustEnvironmentKnown;

#verificationRequirement requirement <'ver-1'> rangeTest { :>> verifyMethod = VerificationMethodKind::test; }
#vvCase verification rangeTestCase {
    objective {
        verify rangeRequirement;
        verify transmitterEnergy;
    }
}
#refinement dependency from rangeTestCase to rangeTest;
```

| Package | What it holds |
| --- | --- |
| `Stakeholders` | the `#stakeholderActor`s: the science team and the lander provider |
| `Expectations` | the `#stakeholderExpectationStatement`s, the `#need`, `#goal` and `#missionObjective`s, their traces and derivations |
| `Measures` | the `#measureOfEffectiveness` and `#measureOfPerformance`s, refining the objectives and requirements |
| `Requirements` | the `#systemRequirement`, `#subsystemRequirement` and `#componentRequirement`s with their derivations |
| `Structure` | the system context, the lidar and its components, the `#tpm` attributes, and what satisfies each requirement |
| `Behavior` | the mission activity and the system functions |
| `Verification and Validation` | the verification and validation requirements, the V&V configuration, event, cases, steps and artifacts |
| `Products` | the Handbook's tables as documents generated from the model |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/hdbk1009-demo/hdbk1009-demo.sysml -validate
```

The model has no errors, and `-validate` also runs the NASA-HDBK-1009A checks, which report the
gaps the model leaves open on purpose (each is marked with a `doc` comment):

| Warning | Gap |
| --- | --- |
| `hdbk1009-requirement-not-derived` | objective `o-2` derives from no goal; measure of performance `mop-2` from no measure of effectiveness; component requirement `cmp-1` from no requirement |
| `hdbk1009-expectation-not-traced` | objective `o-2` is traced to no expectation statement |
| `hdbk1009-objective-not-validated` | no validation requirement satisfies objective `o-2` |
| `hdbk1009-requirement-not-verified` | no V&V case verifies component requirement `cmp-1` |
| `hdbk1009-vv-requirement-not-refined` | no V&V case refines verification requirement `ver-2` |
| `hdbk1009-vv-requirement-missing-method` | verification requirement `ver-2` states no `verifyMethod` |

```text
examples/hdbk1009-demo/hdbk1009-demo.sysml:53:4: warning[hdbk1009-requirement-not-derived]: This objective derives from no goal: the model declares goals, so Figure 7-1 of NASA-HDBK-1009A expects a #d
			#missionObjective requirement <'o-2'> stayWithinAllocation {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/hdbk1009-demo/hdbk1009-demo.sysml:53:4: warning[hdbk1009-objective-not-validated]: This objective is satisfied by no validation requirement: the model declares validation requirements, so Fig
			#missionObjective requirement <'o-2'> stayWithinAllocation {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/hdbk1009-demo/hdbk1009-demo.sysml:53:4: warning[hdbk1009-expectation-not-traced]: This objective is traced to no stakeholder expectation statement: the model declares stakeholder expectation 
			#missionObjective requirement <'o-2'> stayWithinAllocation {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/hdbk1009-demo/hdbk1009-demo.sysml:78:4: warning[hdbk1009-requirement-not-derived]: This measure of performance derives from no measure of effectiveness: the model declares measures of effecti
			#measureOfPerformance requirement <'mop-2'> instrumentMass {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/hdbk1009-demo/hdbk1009-demo.sysml:109:4: warning[hdbk1009-requirement-not-derived]: This component requirement derives from no system requirement: the model declares system requirements, so F
			#componentRequirement requirement <'cmp-1'> laserDiodeCurrent {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/hdbk1009-demo/hdbk1009-demo.sysml:109:4: warning[hdbk1009-requirement-not-verified]: This component requirement is verified by nothing: the model declares verification requirements, so Figure
			#componentRequirement requirement <'cmp-1'> laserDiodeCurrent {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/hdbk1009-demo/hdbk1009-demo.sysml:172:4: warning[hdbk1009-vv-requirement-not-refined]: This verification requirement is refined by no V&V case: the model declares V&V cases, so Figure 7-2 of 
			#verificationRequirement requirement <'ver-2'> massAnalysis {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/hdbk1009-demo/hdbk1009-demo.sysml:172:4: warning[hdbk1009-vv-requirement-missing-method]: This verification requirement states no verifyMethod: the matrices of NASA-HDBK-1009A sections 8.17 a
			#verificationRequirement requirement <'ver-2'> massAnalysis {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
✓ comment
✓ package 'Dust Lidar Pathfinder Model'
✓ examples/hdbk1009-demo/hdbk1009-demo.sysml: no errors
```

State the missing relationship or method and the warning goes away.

Render a work product (Markdown by default; `-doc-form html` or `pdf` for the others):

```bash
./bin/sysml examples/hdbk1009-demo/hdbk1009-demo.sysml \
    -render-document "'Dust Lidar Pathfinder Model'::Products::'Pathfinder Verification Matrices'"
```

`Products` also holds `'Pathfinder Stakeholder Expectations'`, `'Pathfinder Technical Requirements'`
and `'Pathfinder Validation Matrix'`. The verification document has the two tables of sections 8.16
and 8.17, the requirements with the V&V cases verifying them and the verification requirements
with their method, criteria, refining case, artifact, event and closure:

```markdown
# Dust Lidar Pathfinder Verification Matrices

## Requirements verification matrix

*Requirements and the V\&V cases verifying them*

| Id | Name | Text | Verified By | Verify Method |
| --- | --- | --- | --- | --- |
| sys-1 | rangeRequirement | The lidar shall detect dust at the reference density to an altitude of 1 km. | rangeTestCase | test |
| sys-2 | massRequirement | The instrument shall have a mass of no more than 12 kg. | massRollUp | analyze |
| sub-1 | transmitterEnergy | The transmitter shall emit pulses of at least 2 mJ. | rangeTestCase |  |
| cmp-1 | laserDiodeCurrent | The laser diode driver shall supply 40 A pulses. No requirement is its origin and<br>no V\&V case verifies it. |  |  |

## Verification requirements matrix

*Verification requirements*

| Id | Name | Verify Method | Text | Success Criteria | Refined By | Artifact | Verification Event | Closure Status | Closure Date | Closure Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| ver-1 | rangeTest | test | Fire the lidar down a 1 km horizontal range through the reference aerosol and record the return. | The return at 1 km exceeds the detection threshold in 95 of 100 shots. | rangeTestCase | rangeTestReport | instrumentTestCampaign | Open |  |  |
| ver-2 | massAnalysis |  | Sum the measured masses of the flight components. No V\&V case refines this<br>requirement and it states no method. | The sum is no more than 12 kg. |  |  |  |  |  |  |
```
