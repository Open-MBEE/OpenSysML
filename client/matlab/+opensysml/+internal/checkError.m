function answer = checkError(answer, method)
%CHECKERROR Raise 'opensysml:diagnostics' when the answer reports a model
%failure: error non-empty. diagnostics= is a cell of decoded findings.

    if isfield(answer, 'error') && ~isempty(answer.error)
        diags = {};
        if isfield(answer, 'diagnostics') && ~isempty(answer.diagnostics)
            raw = answer.diagnostics;
            if isstruct(raw), raw = num2cell(raw); end
            if iscell(raw), diags = cellfun(@opensysml.internal.decodeDiagnostic, raw, 'UniformOutput', false); end
        end
        % MException cannot carry the list; render it into the message.
        text = char(answer.error);
        for i = 1:numel(diags)
            d = diags{i};
            text = sprintf('%s\n  %s: %s', text, d.severity, d.message);
        end
        error('opensysml:diagnostics', '%s', text);
    end
end
