function values = decodeEvaluations(raw, instances)
%DECODEEVALUATIONS Decode analysis-case function evaluations.

    if nargin < 2, instances = []; end
    raw = toCells(raw);
    values = cell(1, numel(raw));
    for i = 1:numel(raw)
        item = raw{i};
        arguments = decodeValueList(fieldOr(item, 'arguments', {}), instances);
        result = [];
        if isempty(fieldOr(item, 'error', '')) && isfield(item, 'result')
            result = decodeOne(item.result, instances);
        end
        values{i} = struct('functionId', fieldOr(item, 'functionId', ''), ...
            'arguments', {arguments}, 'result', result, ...
            'error', fieldOr(item, 'error', ''), ...
            'selected', logical(fieldOr(item, 'selected', false)), ...
            'tied', logical(fieldOr(item, 'tied', false)));
    end
end

function values = decodeValueList(raw, instances)
    raw = toCells(raw);
    values = cell(1, numel(raw));
    for i = 1:numel(raw), values{i} = decodeOne(raw{i}, instances); end
end

function value = decodeOne(raw, instances)
    try
        if isa(instances, 'containers.Map')
            value = opensysml.decodeValue(raw, ...
                opensysml.internal.instanceResolver(instances));
        else
            value = opensysml.decodeValue(raw);
        end
    catch e, value = e;
    end
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = num2cell(raw(:)');
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
