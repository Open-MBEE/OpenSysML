function result = decodeAnalysisResult(model, raw)
%DECODEANALYSISRESULT Build an AnalysisResult from a RunAnalysisResponse.
    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isfield(raw, 'instances')
        [~, instances] = opensysml.internal.decodeInstances(model.connection, raw.instances);
    end
    outputs = opensysml.internal.decodeOutputEntries(fieldOr(raw, 'outputs', {}), ...
        model.connection, instances);
    verifications = opensysml.internal.decodeVerificationVerdicts( ...
        fieldOr(raw, 'verificationVerdicts', {}));
    verdicts = opensysml.internal.decodeVerdicts(fieldOr(raw, 'verdicts', {}), ...
        instances, diagnostics, verifications);
    evaluations = opensysml.internal.decodeEvaluations(fieldOr(raw, 'evaluations', {}), instances);
    preview = [];
    if isfield(raw, 'toolDryRun') && isstruct(raw.toolDryRun)
        preview = opensysml.internal.decodeToolDryRun(raw.toolDryRun);
    end
    result = opensysml.AnalysisResult(outputs, verdicts, instances, diagnostics, ...
        verifications, evaluations, opensysml.internal.decodeStanding(raw), preview);
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
