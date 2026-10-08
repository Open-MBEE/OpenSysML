function result = decodeExplorationResponse(raw, conn)
%DECODEEXPLORATIONRESPONSE Decode the outcomes and status of an exploration.

    diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(raw, 'diagnostics', {}));
    message = fieldOr(raw, 'error', '');
    if ~isempty(message)
        opensysml.internal.raiseFailure(message, fieldOr(raw, 'failureReason', ''), diagnostics);
    end
    status = fieldOr(raw, 'exploration', struct());
    rawOutcomes = toCells(fieldOr(raw, 'outcomes', {}));
    outcomes = cell(1, numel(rawOutcomes));
    for i = 1:numel(rawOutcomes)
        outcomes{i} = opensysml.internal.decodeOutcome(rawOutcomes{i}, conn);
    end
    result = opensysml.Exploration(outcomes, ...
        fieldOr(status, 'complete', true), fieldOr(status, 'runs', 0), ...
        fieldOr(status, 'budgetsHit', {}), fieldOr(status, 'runsBudget', 0), ...
        fieldOr(status, 'depthBudget', 0), ...
        fieldOr(status, 'probabilitiesLowerBound', false));
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {};
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
