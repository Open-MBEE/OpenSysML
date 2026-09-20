package org.openmbee.opensysml.syson.run;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.syson.sysml.Element;
import org.openmbee.opensysml.Analysis;
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

@Service
public class RunWithOpenSysMLService {
    private final Connection connection;
    private final ProjectExporter exporter;
    private final RunResultStore store;
    private final OpenSysMLProperties properties;

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
        ElementIndexTarget selected = project.index().byElement(target).map(value -> new ElementIndexTarget(value))
                .orElse(null);
        if (selected == null) {
            RunResult result = RunResult.failure("", input.operation(), "", "selected element has no qualified name in the export");
            store.put(context.getId(), result);
            return result;
        }
        String targetName = selected.value.qualifiedName();
        List<DiagnosticMapper.Mapped> mappedDiagnostics = new ArrayList<>();
        List<RunDiagnostic> exportDiagnostics = project.messages().stream()
                .map(message -> new RunDiagnostic(message.level().name().toLowerCase(), message.message(), "", null, null,
                        targetName, selected.value.elementId(), selected.value.siriusId()))
                .toList();
        try {
            Model model = connection.parseSources(project.documents());
            model.diagnostics().forEach(diagnostic -> mappedDiagnostics.add(DiagnosticMapper.map(diagnostic, project)));
            ExecutionOptions options = options(input);
            ResultParts parts = dispatch(model, targetName, input, options);
            parts.schedule = options.schedule().orElse(null);
            for (Diagnostic diagnostic : parts.diagnostics) mappedDiagnostics.add(DiagnosticMapper.map(diagnostic, project));
            RunResult result = result(model.hash(), input, targetName, parts, mappedDiagnostics, exportDiagnostics, project);
            store.put(context.getId(), result);
            return result;
        } catch (ModelException | ServiceException | CapabilityException | IllegalArgumentException exception) {
            RunDiagnostic diagnostic = new RunDiagnostic("error", exception.getMessage(), "", null, null, targetName,
                    selected.value.elementId(), selected.value.siriusId());
            List<RunDiagnostic> diagnostics = new ArrayList<>(exportDiagnostics);
            mappedDiagnostics.forEach(value -> diagnostics.add(value.diagnostic()));
            diagnostics.add(diagnostic);
            RunResult result = new RunResult("", input.operation(), targetName, false, null, null, null, List.of(),
                    List.of(), null, diagnostics, List.of(), List.of(), Map.of());
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

    private ResultParts dispatch(Model model, String target, RunWithOpenSysMLInput input, ExecutionOptions options) {
        Map<String, Value> values = new LinkedHashMap<>();
        input.inputs().forEach((name, expression) -> values.put(name, model.eval(expression)));
        List<Value> arguments = input.arguments().stream().map(model::eval).toList();
        return switch (input.operation()) {
            case INSTANTIATE -> ResultParts.instantiation(model.instantiate(target));
            case EXECUTE_ACTION -> ResultParts.action(model.executeAction(target, values, options));
            case EXPLORE_ACTION -> ResultParts.exploration(model.exploreAction(target, values, options));
            case EXECUTE_STATE -> ResultParts.state(model.executeState(target, input.events(), options));
            case EXPLORE_STATE -> ResultParts.exploration(model.exploreState(target, input.events(), options));
            case VERIFY_CONSTRAINT -> ResultParts.verification(input.subject() == null ? model.verifyConstraint(target)
                    : model.verifyConstraint(target, input.subject()));
            case VERIFY_REQUIREMENT -> ResultParts.verification(input.subject() == null ? model.verifyRequirement(target)
                    : model.verifyRequirement(target, input.subject()));
            case VERIFY_SATISFACTION -> ResultParts.satisfaction(
                    input.subject() == null ? model.verifySatisfaction() : model.verifySatisfaction(input.subject()));
            case EVALUATE_CALC -> ResultParts.calculation(model.evaluateCalc(target, arguments));
            case RUN_ANALYSIS -> ResultParts.analysis(model.runAnalysis(target));
            case VALIDATE_INSTANCE -> ResultParts.validation(model.validateInstance(target));
        };
    }

    private RunResult result(String hash, RunWithOpenSysMLInput input, String target, ResultParts parts,
            List<DiagnosticMapper.Mapped> mapped, List<RunDiagnostic> exportDiagnostics, ExportedProject project) {
        List<RunDiagnostic> diagnostics = new ArrayList<>();
        diagnostics.addAll(exportDiagnostics);
        Map<RunDiagnostic, Element> elements = new LinkedHashMap<>();
        mapped.forEach(value -> { diagnostics.add(value.diagnostic()); elements.put(value.diagnostic(), value.element()); });
        diagnostics.addAll(parts.extraDiagnostics.stream().map(d -> {
            RunDiagnostic value = new RunDiagnostic("error", d, "", null, null, target, null, null);
            return value;
        }).toList());
        return new RunResult(hash, input.operation(), target, parts.ok, parts.verdict, parts.schedule, parts.finalTime,
                parts.outputs, parts.trace, parts.resultText, diagnostics, parts.verdicts, parts.instances, elements);
    }

    private static final class ElementIndexTarget {
        private final org.openmbee.opensysml.syson.identity.ElementIndex.IndexedElement value;
        private ElementIndexTarget(org.openmbee.opensysml.syson.identity.ElementIndex.IndexedElement value) { this.value = value; }
    }

    private static final class ResultParts {
        private boolean ok = true;
        private String verdict;
        private String schedule;
        private Double finalTime;
        private List<RunNamedValue> outputs = List.of();
        private List<String> trace = List.of();
        private String resultText;
        private List<Diagnostic> diagnostics = List.of();
        private List<String> extraDiagnostics = List.of();
        private List<RunVerdict> verdicts = List.of();
        private List<RunInstance> instances = List.of();
        static ResultParts instantiation(Instantiation result) {
            ResultParts p = new ResultParts(); p.diagnostics = result.diagnostics(); p.instances = instances(result.reachable());
            return p;
        }
        static ResultParts action(ActionRun result) { ResultParts p = new ResultParts(); p.outputs = values(result.outputs()); p.finalTime = result.finalTime().isPresent() ? result.finalTime().getAsDouble() : null; p.diagnostics = result.diagnostics(); return p; }
        static ResultParts state(StateRun result) { ResultParts p = new ResultParts(); p.outputs = values(result.finalContext()); p.trace = result.statesVisited(); p.finalTime = result.finalTime().isPresent() ? result.finalTime().getAsDouble() : null; p.diagnostics = result.diagnostics(); return p; }
        static ResultParts exploration(Exploration result) { ResultParts p = new ResultParts(); p.ok = result.complete(); p.verdict = result.status(); if (!result.outcomes().isEmpty()) { var o = result.outcomes().get(0); p.outputs = values(o.outputs()); p.trace = o.statesVisited(); p.diagnostics = o.diagnostics(); } return p; }
        static ResultParts verification(Verification result) { ResultParts p = new ResultParts(); p.verdict = result.verdict().decided() ? (result.verdict().holds() ? "holds" : "violated") : "undecided"; p.diagnostics = result.diagnostics(); p.verdicts = List.of(verdict(result.verdict())); p.instances = instances(result.instances()); return p; }
        static ResultParts satisfaction(Satisfaction result) { ResultParts p = new ResultParts(); p.verdict = result.holds() ? "pass" : "fail"; p.verdicts = result.verdicts().stream().map(ResultParts::verdict).toList(); p.diagnostics = result.diagnostics(); p.instances = instances(result.instances()); return p; }
        static ResultParts calculation(Calculation result) { ResultParts p = new ResultParts(); p.resultText = result.value().map(ValueText::render).orElse(null); p.outputs = values(result.outputs()); p.diagnostics = result.diagnostics(); return p; }
        static ResultParts analysis(Analysis result) { ResultParts p = new ResultParts(); p.verdict = result.holds() ? "holds" : "violated"; p.diagnostics = result.diagnostics(); p.instances = instances(result.instances()); return p; }
        static ResultParts validation(org.openmbee.opensysml.Validation result) { ResultParts p = new ResultParts(); p.verdict = result.holds() ? "holds" : "violated"; p.verdicts = result.verdicts().stream().map(ResultParts::verdict).toList(); p.diagnostics = result.diagnostics(); p.instances = instances(result.instances()); return p; }
        static List<RunNamedValue> values(Map<String, Value> values) { return values.entrySet().stream().map(e -> new RunNamedValue(e.getKey(), ValueText.render(e.getValue()))).toList(); }
        static List<RunInstance> instances(List<Instance> instances) { return instances.stream().map(instance -> new RunInstance(instance.id(), instance.typeSymbolId(), null, instance.featureValues().values().stream().flatMap(value -> value.value().stream()).map(value -> new RunNamedValue("value", ValueText.render(value))).toList())).toList(); }
        static RunVerdict verdict(Verdict value) { return new RunVerdict(value.element(), value.kind(), value.holds(), value.error().orElse(null), null); }
    }
}
