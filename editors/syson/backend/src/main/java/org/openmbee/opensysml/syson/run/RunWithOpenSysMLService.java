package org.openmbee.opensysml.syson.run;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.syson.sysml.Element;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.AnalysisOptions;
import org.openmbee.opensysml.Calculation;
import org.openmbee.opensysml.CapabilityException;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.ExecutionOptions;
import org.openmbee.opensysml.Exploration;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.ModelException;
import org.openmbee.opensysml.ServiceException;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.StateRun;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.Verification;
import org.openmbee.opensysml.ActionRun;
import org.openmbee.opensysml.Connection;
import org.springframework.stereotype.Service;
import org.openmbee.opensysml.syson.OpenSysMLProperties;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.export.ProjectExporter;
import org.openmbee.opensysml.syson.export.ProjectTextExporter;
import org.openmbee.opensysml.syson.identity.ElementIndex.IndexedElement;
import org.springframework.beans.factory.annotation.Autowired;

@Service
public class RunWithOpenSysMLService {
    private final Connection connection;
    private final ProjectExporter exporter;
    private final RunResultStore store;
    private final OpenSysMLProperties properties;

    @Autowired
    public RunWithOpenSysMLService(Connection connection, ProjectTextExporter exporter, RunResultStore store,
            OpenSysMLProperties properties) {
        this(connection, (ProjectExporter) exporter, store, properties);
    }

    public RunWithOpenSysMLService(Connection connection, ProjectExporter exporter, RunResultStore store,
            OpenSysMLProperties properties) {
        this.connection = connection;
        this.exporter = exporter;
        this.store = store;
        this.properties = properties;
    }

    public RunResult run(IEMFEditingContext context, Element target, RunWithOpenSysMLInput input) {
        ExportedProject project = exporter.export(context);
        IndexedElement selected = project.index().byElement(target).orElse(null);
        if (selected == null) {
            RunResult result = RunResult.failure("", input.operation(), "", "selected element has no qualified name in the export");
            store.put(context.getId(), result);
            return result;
        }
        String targetName = selected.qualifiedName();
        List<DiagnosticMapper.Mapped> mappedDiagnostics = new ArrayList<>();
        List<RunDiagnostic> exportDiagnostics = project.messages().stream()
                .map(message -> new RunDiagnostic(message.level().name().toLowerCase(), message.message(), "", null, null,
                        targetName, selected.elementId(), selected.siriusId()))
                .toList();
        try {
            Model model = connection.parseSources(project.documents());
            model.diagnostics().forEach(diagnostic -> mappedDiagnostics.add(DiagnosticMapper.map(diagnostic, project)));
            ExecutionOptions options = options(input);
            ResultParts parts = dispatch(model, targetName, input, options, project);
            parts.schedule = options.schedule().orElse(null);
            for (Diagnostic diagnostic : parts.diagnostics) mappedDiagnostics.add(DiagnosticMapper.map(diagnostic, project));
            RunResult result = result(model.hash(), input, targetName, parts, mappedDiagnostics, exportDiagnostics);
            store.put(context.getId(), result);
            return result;
        } catch (ModelException | ServiceException | CapabilityException | IllegalArgumentException exception) {
            RunDiagnostic diagnostic = new RunDiagnostic("error", exception.getMessage(), "", null, null, targetName,
                    selected.elementId(), selected.siriusId());
            List<RunResult.MappedDiagnostic> resultDiagnostics = new ArrayList<>();
            exportDiagnostics.forEach(value -> resultDiagnostics.add(new RunResult.MappedDiagnostic(value, null)));
            mappedDiagnostics.forEach(value -> resultDiagnostics.add(
                    new RunResult.MappedDiagnostic(value.diagnostic(), value.element())));
            resultDiagnostics.add(new RunResult.MappedDiagnostic(diagnostic, selected.element()));
            RunResult result = new RunResult("", input.operation(), targetName, false, null, null, null, List.of(),
                    List.of(), null, List.of(), List.of(), List.of(),
                    resultDiagnostics);
            store.put(context.getId(), result);
            return result;
        }
    }

    private ExecutionOptions options(RunWithOpenSysMLInput input) {
        ExecutionOptions options = ExecutionOptions.defaults();
        String schedule = input.schedule();
        if (schedule == null && input.operation() != RunOperation.EXPLORE_ACTION
                && input.operation() != RunOperation.EXPLORE_STATE) schedule = properties.getSchedule();
        if ((input.operation() == RunOperation.EXPLORE_ACTION || input.operation() == RunOperation.EXPLORE_STATE)
                && (schedule == null || !schedule.equals("explore") && !schedule.startsWith("explore:"))) {
            schedule = properties.explorationSchedule();
        }
        if (schedule != null) options = options.withSchedule(schedule);
        if (properties.getPerformer() != null) options = options.withPerformer(properties.getPerformer());
        return options;
    }

    private ResultParts dispatch(Model model, String target, RunWithOpenSysMLInput input, ExecutionOptions options,
            ExportedProject project) {
        Map<String, Value> values = new LinkedHashMap<>();
        input.inputs().forEach((name, expression) -> values.put(name, model.eval(expression)));
        List<Value> arguments = input.arguments().stream().map(model::eval).toList();
        return switch (input.operation()) {
            case INSTANTIATE -> ResultParts.instantiation(model.instantiate(target), project);
            case EXECUTE_ACTION -> ResultParts.action(model.executeAction(target, values, options), project);
            case EXPLORE_ACTION -> ResultParts.exploration(model.exploreAction(target, values, options), project);
            case EXECUTE_STATE -> ResultParts.state(model.executeState(target, input.events(), options), project);
            case EXPLORE_STATE -> ResultParts.exploration(model.exploreState(target, input.events(), options), project);
            case VERIFY_CONSTRAINT -> ResultParts.verification(input.subject() == null ? model.verifyConstraint(target)
                    : model.verifyConstraint(target, input.subject()), project);
            case VERIFY_REQUIREMENT -> ResultParts.verification(input.subject() == null ? model.verifyRequirement(target)
                    : model.verifyRequirement(target, input.subject()), project);
            case VERIFY_SATISFACTION -> ResultParts.satisfaction(
                    input.subject() == null ? model.verifySatisfaction() : model.verifySatisfaction(input.subject()),
                    project);
            case EVALUATE_CALC -> ResultParts.calculation(model.evaluateCalc(target, arguments), project);
            case RUN_ANALYSIS -> ResultParts.analysis(model.runAnalysis(target,
                    new AnalysisOptions(Optional.ofNullable(input.subject()), arguments, values,
                            options.schedule())), project);
            case VALIDATE_INSTANCE -> ResultParts.validation(model.validateInstance(target), project);
        };
    }

    private RunResult result(String hash, RunWithOpenSysMLInput input, String target, ResultParts parts,
            List<DiagnosticMapper.Mapped> mapped, List<RunDiagnostic> exportDiagnostics) {
        List<RunResult.MappedDiagnostic> mappedDiagnostics = new ArrayList<>();
        exportDiagnostics.forEach(value -> mappedDiagnostics.add(new RunResult.MappedDiagnostic(value, null)));
        mapped.forEach(value -> {
            mappedDiagnostics.add(new RunResult.MappedDiagnostic(value.diagnostic(), value.element()));
        });
        return new RunResult(hash, input.operation(), target, parts.ok, parts.verdict, parts.schedule, parts.finalTime,
                parts.outputs, parts.trace, parts.resultText, parts.outcomes, parts.verdicts, parts.instances,
                mappedDiagnostics);
    }

    static final class ResultParts {
        private final ExportedProject project;
        private boolean ok = true;
        private String verdict;
        private String schedule;
        private Double finalTime;
        private List<RunNamedValue> outputs = List.of();
        private List<String> trace = List.of();
        private List<RunOutcome> outcomes = List.of();
        private String resultText;
        private List<Diagnostic> diagnostics = List.of();
        private List<RunVerdict> verdicts = List.of();
        private List<RunInstance> instances = List.of();

        private ResultParts(ExportedProject project) {
            this.project = project;
        }

        String verdict() {
            return verdict;
        }

        boolean ok() {
            return ok;
        }

        String resultText() {
            return resultText;
        }

        List<RunOutcome> outcomes() {
            return outcomes;
        }

        List<RunNamedValue> outputs() {
            return outputs;
        }

        List<RunVerdict> verdicts() {
            return verdicts;
        }

        static ResultParts instantiation(Instantiation result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.diagnostics = result.diagnostics();
            p.instances = p.instances(result.reachable());
            return p;
        }
        static ResultParts action(ActionRun result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.outputs = values(result.outputs());
            p.finalTime = result.finalTime().isPresent() ? result.finalTime().getAsDouble() : null;
            p.diagnostics = result.diagnostics();
            return p;
        }
        static ResultParts state(StateRun result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.outputs = values(result.finalContext());
            p.trace = result.statesVisited();
            p.finalTime = result.finalTime().isPresent() ? result.finalTime().getAsDouble() : null;
            p.diagnostics = result.diagnostics();
            return p;
        }
        static ResultParts exploration(Exploration result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.ok = result.complete() && result.outcomes().stream().allMatch(outcome -> outcome.error().isEmpty());
            p.verdict = result.status();
            p.outcomes = result.outcomes().stream().map(outcome -> new RunOutcome(values(outcome.outputs()),
                    outcome.finalState().orElse(null), outcome.statesVisited(), outcome.error().orElse(null),
                    outcome.linearizations(), outcome.witness())).toList();
            p.diagnostics = result.outcomes().stream().flatMap(outcome -> outcome.diagnostics().stream()).toList();
            p.resultText = result.status();
            long failed = result.outcomes().stream().filter(outcome -> outcome.error().isPresent()).count();
            if (failed > 0) {
                p.resultText += "; " + failed + " of " + result.outcomes().size() + " outcomes failed";
            }
            if (!result.outcomes().isEmpty()) {
                var outcome = result.outcomes().get(0);
                p.outputs = values(outcome.outputs());
                p.trace = outcome.statesVisited();
            }
            return p;
        }
        static ResultParts verification(Verification result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.verdict = result.verdict().decided()
                    ? (result.verdict().holds() ? "holds" : "violated")
                    : "undecided";
            p.diagnostics = result.diagnostics();
            p.verdicts = List.of(p.verdict(result.verdict()));
            p.instances = p.instances(result.instances());
            return p;
        }
        static ResultParts satisfaction(Satisfaction result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.verdicts = result.verdicts().stream().map(p::verdict).toList();
            p.verdict = result.verdicts().stream().anyMatch(verdict -> !verdict.decided())
                    ? "undecided" : result.holds() ? "pass" : "fail";
            p.diagnostics = result.diagnostics();
            p.instances = p.instances(result.instances());
            return p;
        }
        static ResultParts calculation(Calculation result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.resultText = result.value().map(ValueText::render).orElse(null);
            p.outputs = values(result.outputs());
            p.diagnostics = result.diagnostics();
            return p;
        }
        static ResultParts analysis(Analysis result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.verdict = result.verdicts().stream().anyMatch(verdict -> !verdict.decided())
                    ? "undecided" : result.holds() ? "holds" : "violated";
            p.outputs = values(result.outputs());
            p.verdicts = result.verdicts().stream().map(p::verdict).toList();
            p.diagnostics = result.diagnostics();
            p.instances = p.instances(result.instances());
            return p;
        }
        static ResultParts validation(org.openmbee.opensysml.Validation result, ExportedProject project) {
            ResultParts p = new ResultParts(project);
            p.verdicts = result.verdicts().stream().map(p::verdict).toList();
            p.verdict = result.summary().decided()
                    ? (result.holds() ? "holds" : "violated") : "undecided";
            p.diagnostics = result.diagnostics();
            p.instances = p.instances(result.instances());
            return p;
        }
        static List<RunNamedValue> values(Map<String, Value> values) {
            return values.entrySet().stream()
                    .map(entry -> new RunNamedValue(entry.getKey(), ValueText.render(entry.getValue())))
                    .toList();
        }
        List<RunInstance> instances(List<Instance> instances) {
            return instances.stream().map(instance -> {
                String siriusId = project.index().byQualifiedName(instance.typeSymbolId())
                        .map(IndexedElement::siriusId).orElse(null);
                List<RunNamedValue> featureValues = instance.featureValues().values().stream()
                        .map(value -> new RunNamedValue(value.featureName(),
                                value.value().map(ValueText::render).orElseGet(
                                        () -> value.values().stream().map(ValueText::render).toList().toString())))
                        .toList();
                return new RunInstance(instance.id(), instance.typeSymbolId(), siriusId, featureValues);
            }).toList();
        }
        RunVerdict verdict(Verdict value) {
            String siriusId = value.elementId().flatMap(name -> project.index().byQualifiedName(name))
                    .map(IndexedElement::siriusId).orElse(null);
            return new RunVerdict(value.element(), value.kind(), value.holds(), value.decided(),
                    value.error().orElse(null), siriusId);
        }
    }
}
