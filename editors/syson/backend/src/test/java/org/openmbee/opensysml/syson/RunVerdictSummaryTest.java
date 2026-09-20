package org.openmbee.opensysml.syson.run;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Optional;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.FailureReason;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.Validation;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.identity.ElementIndex;

import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService.ResultParts;

class RunVerdictSummaryTest {
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
                List.of()), PROJECT).verdict()).isEqualTo("undecided");
    }

    @Test
    void mapsValidationDecidedAndUndecidedSummaries() {
        assertThat(ResultParts.validation(new Validation(verdict(true, false), List.of(), List.of(), List.of(),
                List.of(), true), PROJECT).verdict()).isEqualTo("holds");
        assertThat(ResultParts.validation(new Validation(verdict(false, false), List.of(), List.of(), List.of(),
                List.of(), true), PROJECT).verdict()).isEqualTo("violated");
        assertThat(ResultParts.validation(new Validation(verdict(false, true), List.of(), List.of(), List.of(),
                List.of(), true), PROJECT).verdict()).isEqualTo("undecided");
    }

    @Test
    void mapsAnalysisDecidedAndUndecidedSummaries() {
        assertThat(ResultParts.analysis(new Analysis(java.util.Map.of(), List.of(verdict(true, false)), List.of(),
                List.of(), List.of(), List.of(), STANDING), PROJECT).verdict()).isEqualTo("holds");
        assertThat(ResultParts.analysis(new Analysis(java.util.Map.of(), List.of(verdict(false, false)), List.of(),
                List.of(), List.of(), List.of(), STANDING), PROJECT).verdict()).isEqualTo("violated");
        assertThat(ResultParts.analysis(new Analysis(java.util.Map.of(), List.of(verdict(false, true)), List.of(),
                List.of(), List.of(), List.of(), STANDING), PROJECT).verdict()).isEqualTo("undecided");
    }

    private static Verdict verdict(boolean holds, boolean undecided) {
        return new Verdict(Verdict.KIND_REQUIREMENT, Optional.of("Req"), "Req", holds, Optional.empty(),
                Optional.empty(), Optional.empty(), undecided ? Optional.of("unknown") : Optional.empty(),
                FailureReason.EVALUATION, Optional.empty(), Optional.empty(), STANDING);
    }
}
