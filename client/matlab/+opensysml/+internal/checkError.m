function answer = checkError(answer, method, details)
%CHECKERROR Raise a typed diagnostics error when the model reports failure.

    if nargin < 3 || isempty(details), details = struct(); end
    if isfield(answer, 'error') && ~isempty(answer.error)
        diags = {};
        if isfield(answer, 'diagnostics') && ~isempty(answer.diagnostics)
            raw = answer.diagnostics;
            if isstruct(raw), raw = num2cell(raw); end
            if iscell(raw), diags = cellfun(@opensysml.internal.decodeDiagnostic, raw, 'UniformOutput', false); end
        end
        text = char(answer.error);
        for i = 1:numel(diags)
            d = diags{i};
            text = sprintf('%s\n  %s: %s', text, d.severity, d.message);
        end
        failure = '';
        if isfield(answer, 'failureReason'), failure = char(answer.failureReason); end
        details.diagnostics = diags;
        if ~isempty(failure), details.failure = failure; end
        identifier = 'opensysml:diagnostics:execution';
        if strcmp(failure, 'FAILURE_REASON_WRONG_KIND')
            identifier = 'opensysml:diagnostics:wrongKind';
        elseif strcmp(method, 'ParseSources')
            identifier = 'opensysml:diagnostics:model';
        elseif strcmp(method, 'Convert')
            identifier = 'opensysml:diagnostics:conversion';
        elseif strcmp(method, 'Migrate')
            identifier = 'opensysml:diagnostics:migration';
        end
        opensysml.internal.raise(identifier, text, diags, details);
    end
end
