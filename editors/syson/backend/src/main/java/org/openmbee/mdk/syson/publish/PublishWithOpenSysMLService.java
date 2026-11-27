package org.openmbee.mdk.syson.publish;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.function.Function;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.openmbee.mdk.syson.OpenSysMLProperties;
import org.openmbee.mdk.syson.export.ExportedProject;
import org.openmbee.mdk.syson.export.ProjectExporter;
import org.openmbee.mdk.syson.export.ProjectTextExporter;
import org.openmbee.mdk.syson.run.DiagnosticMapper;
import org.openmbee.mdk.syson.run.RunDiagnostic;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.repository.ApiConfig;
import org.openmbee.opensysml.repository.Publisher;
import org.openmbee.opensysml.repository.SysmlApiClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

/**
 * Publishes an editing context's model to a project branch of the configured SysML v2 API server.
 * A model with errors is refused, since it would publish a partial model as the whole.
 */
@Service
public class PublishWithOpenSysMLService {
    private final Connection connection;
    private final ProjectExporter exporter;
    private final OpenSysMLProperties properties;
    private final Map<String, String> environment;
    private final Function<ApiConfig, Publisher> publishers;

    @Autowired
    public PublishWithOpenSysMLService(Connection connection, ProjectTextExporter exporter,
            OpenSysMLProperties properties) {
        this(connection, exporter, properties, System.getenv(), config -> new Publisher(new SysmlApiClient(config)));
    }

    public PublishWithOpenSysMLService(Connection connection, ProjectExporter exporter, OpenSysMLProperties properties,
            Map<String, String> environment, Function<ApiConfig, Publisher> publishers) {
        this.connection = connection;
        this.exporter = exporter;
        this.properties = properties;
        this.environment = Map.copyOf(environment);
        this.publishers = publishers;
    }

    /** The server every publication goes to; the browser names a project and branch, never a host. */
    public String server() {
        return properties.getPublish().apiConfig(environment).baseUrl();
    }

    public PublishResult publish(IEMFEditingContext context, PublishWithOpenSysMLInput input) {
        ApiConfig config = properties.getPublish().apiConfig(environment);
        if (!input.token().isEmpty()) config = config.withToken(input.token());
        // Refuses a URL the token may not travel to before anything is exported.
        config.checkedBase();
        String server = config.baseUrl();
        if (input.project().isEmpty() && input.projectName().isEmpty()) {
            throw new IllegalArgumentException("a publication names a project id, or the name of the project to create");
        }

        ExportedProject project = exporter.export(context);
        List<RunDiagnostic> diagnostics = new ArrayList<>();
        project.messages().forEach(message -> diagnostics.add(new RunDiagnostic(severity(message.level()),
                message.message(), "", null, null, null, null, null)));
        Model model = connection.parseSources(project.documents());
        for (Diagnostic diagnostic : model.diagnostics()) {
            diagnostics.add(DiagnosticMapper.map(diagnostic, project).diagnostic());
        }
        List<RunDiagnostic> errors = diagnostics.stream().filter(value -> value.severity().equals("error")).toList();
        if (!errors.isEmpty()) {
            return PublishResult.refused(server, input, "The model has " + errors.size()
                    + " error(s) and was not published; the first: " + errors.get(0).message(), diagnostics);
        }

        String converted = model.convert("api-json").content();
        Publisher.LastSeen lastSeen = (projectId, branchId) -> Optional.ofNullable(input.lastSeenCommit());
        Publisher.Target target = new Publisher.Target(input.project(), input.projectName(), input.branch(), lastSeen,
                input.deleteMissing(), input.message());
        try {
            return PublishResult.published(server, publishers.apply(config).publish(converted, target), diagnostics);
        } catch (Publisher.BranchMovedException moved) {
            return PublishResult.moved(server, input, moved.getMessage(), diagnostics);
        }
    }

    private static String severity(MessageLevel level) {
        return switch (level) {
            case ERROR -> "error";
            case WARNING -> "warning";
            default -> "info";
        };
    }
}
