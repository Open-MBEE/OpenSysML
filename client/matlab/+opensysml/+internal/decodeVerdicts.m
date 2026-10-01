function values = decodeVerdicts(raw, instances, diagnostics, verifications, allVerifications)
%DECODEVERDICTS Decode a repeated Verdict field.

    if nargin < 2, instances = []; end
    if nargin < 3, diagnostics = {}; end
    if nargin < 4, verifications = {}; end
    if nargin < 5, allVerifications = false; end
    raw = toCells(raw);
    values = cell(1, numel(raw));
    for i = 1:numel(raw)
        item = raw{i};
        if isfield(item, 'failureReason') && ...
                strcmp(item.failureReason, 'FAILURE_REASON_WRONG_KIND')
            opensysml.internal.raise('opensysml:diagnostics:wrongKind', ...
                fieldOr(item, 'error', 'the requested symbol has the wrong kind'), diagnostics);
        end
        ownVerifications = {};
        requirement = fieldOr(item, 'requirementId', '');
        if allVerifications
            ownVerifications = verifications;
        elseif ~isempty(requirement)
            for j = 1:numel(verifications)
                verification = verifications{j};
                if strcmp(verification.requirementId, requirement)
                    ownVerifications{end+1} = verification;
                end
            end
        end
        values{i} = opensysml.Verdict(item, instances, diagnostics, ownVerifications);
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
