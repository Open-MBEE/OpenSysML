function values = decodeVerificationVerdicts(raw)
%DECODEVERIFICATIONVERDICTS Decode verification-case answers.

    raw = toCells(raw);
    values = cell(1, numel(raw));
    for i = 1:numel(raw)
        item = raw{i};
        kind = fieldOr(item, 'kind', '');
        mark = '?';
        if strcmp(kind, 'pass')
            mark = opensysml.internal.unicodeChar(10003);
        elseif strcmp(kind, 'fail')
            mark = opensysml.internal.unicodeChar(10007);
        end
        line = sprintf('%s verification %s verdict: %s', mark, ...
            fieldOr(item, 'caseId', ''), kind);
        subcase = logical(fieldOr(item, 'subcase', false));
        if subcase, line = [line ' (subcase)']; end
        detail = fieldOr(item, 'detail', '');
        if ~isempty(detail)
            line = [line ' ' opensysml.internal.unicodeChar(8212) ' ' detail];
        end
        values{i} = struct('caseId', fieldOr(item, 'caseId', ''), ...
            'kind', kind, 'detail', detail, 'subcase', subcase, ...
            'requirementId', fieldOr(item, 'requirementId', ''), ...
            'explanation', line);
    end
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
