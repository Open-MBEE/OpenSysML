package org.openmbee.opensysml.syson.run;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Optional;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.Exploration;
import org.openmbee.opensysml.FailureReason;
import org.openmbee.opensysml.Outcome;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.Validation;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.identity.ElementIndex;

import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService.ResultParts;

class RunVerdictSummaryTest {
    private static final String UNDECIDED = "undecided";
    private static final ExportedProject PROJECT = new ExportedProject(List.of(), new ElementIndex(java.util.Map.of()),
            List.of(), List.of());
    private static final Standing STANDING = Standing.none();

    @Test
    void mapsSatisfactionDecidedAndUndecidedSummaries() {
        assertThat(ResultParts.satisfaction(new Satisfaction(List.of(verdict(true, false)), List.of(), List.of(),
                List.of()), PROJECT).verdict()).isEqualTo("pass");
        assertThat(ResultParts.satisfaction(new Satisfaction(List.of(verdict(false, false)), List.of(), List.of(),
                List.of()), PROJECT).verdict()).isEqualTo("fail");
        assertThat(ResultParts.satisfaction(new Satisfaction(List.of(verdict(false, true)), List.of(), List.of(),
                List.of()), PROJECT).verdict()).isEqualTo(UNDECIDED);
    }

    @Test
    void mapsValidationDecidedAndUndecidedSummaries() {
        assertThat(ResultParts.validation(new Validation(verdict(true, false), List.of(), List.of(), List.of(),
                List.of(), true), PROJECT).verdict()).isEqualTo("holds");
        assertThat(ResultParts.validation(new Validation(verdict(false, false), List.of(), List.of(), List.of(),
                List.of(), true), PROJECT).verdict()).isEqualTo("violated");
        assertThat(ResultParts.validation(new Validation(verdict(false, true), List.of(), List.of(), List.of(),
                List.of(), true), PROJECT).verdict()).isEqualTo(UNDECIDED);
    }

    @Test
    void mapsAnalysisDecidedAndUndecidedSummaries() {
        ResultParts analysis = ResultParts.analysis(new Analysis(java.util.Map.of("score", new Value.IntegerValue(7)),
                List.of(verdict(true, false)), List.of(), List.of(), List.of(), List.of(), STANDING), PROJECT);
        assertThat(analysis.verdict()).isEqualTo("holds");
        assertThat(analysis.outputs()).extracting("name").containsExactly("score");
        assertThat(analysis.verdicts()).hasSize(1);
        assertThat(ResultParts.analysis(new Analysis(java.util.Map.of(), List.of(verdict(false, false)), List.of(),
                List.of(), List.of(), List.of(), STANDING), PROJECT).verdict()).isEqualTo("violated");
        assertThat(ResultParts.analysis(new Analysis(java.util.Map.of(), List.of(verdict(false, true)), List.of(),
                List.of(), List.of(), List.of(), STANDING), PROJECT).verdict()).isEqualTo(UNDECIDED);
    }

    @Test
    void mapsExplorationFailureInCompleteRun() {
        Outcome completed = new Outcome(java.util.Map.of("value", new Value.IntegerValue(1)), Optional.empty(),
                List.of(), Optional.empty(), 1, 0.5, List.of(), List.of());
        Outcome failed = new Outcome(java.util.Map.of(), Optional.empty(), List.of(), Optional.of("boom"), 1,
                0.0, List.of(), List.of());

        ResultParts parts = ResultParts.exploration(new Exploration(List.of(completed, failed), true, 2, List.of(), 10,
                10, false), PROJECT);

        assertThat(parts.ok()).isFalse();
        assertThat(parts.resultText()).isEqualTo("complete (2 runs); 1 of 2 outcomes failed");
        assertThat(parts.outcomes()).hasSize(2);
    }

    @Test
    void resolvesVerdictByElementId() {
        org.openmbee.opensysml.syson.FakeElement element = new org.openmbee.opensysml.syson.FakeElement(
                "Demo::Vehicle::massPositive");
        ExportedProject project = new ExportedProject(List.of(),
                new ElementIndex(java.util.Map.of(element.getQualifiedName(),
                        new ElementIndex.IndexedElement(element.getQualifiedName(), element.getElementId(), "sid",
                                element))),
                List.of(), List.of());

        ResultParts result = ResultParts.satisfaction(new Satisfaction(List.of(new Verdict(Verdict.KIND_CONSTRAINT,
                Optional.of(element.getQualifiedName()), "massPositive", true, Optional.empty(), Optional.empty(),
                Optional.empty(), Optional.empty(), FailureReason.UNSPECIFIED, Optional.empty(), Optional.empty(),
                STANDING)), List.of(), List.of(), List.of()), project);

        assertThat(result.verdicts()).singleElement().extracting("siriusId").isEqualTo("sid");
    }

    private static Verdict verdict(boolean holds, boolean undecided) {
        return new Verdict(Verdict.KIND_REQUIREMENT, Optional.of("Req"), "Req", holds, Optional.empty(),
                Optional.empty(), Optional.empty(), undecided ? Optional.of("unknown") : Optional.empty(),
                FailureReason.EVALUATION, Optional.empty(), Optional.empty(), STANDING);
    }
}
