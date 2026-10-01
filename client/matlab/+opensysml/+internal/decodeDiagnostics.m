function values = decodeDiagnostics(raw)
%DECODEDIAGNOSTICS Decode a repeated Diagnostic field.

    if isempty(raw), values = {};
    elseif iscell(raw)
        values = cellfun(@opensysml.internal.decodeDiagnostic, raw(:)', 'UniformOutput', false);
    elseif isstruct(raw)
        values = arrayfun(@opensysml.internal.decodeDiagnostic, raw(:)', 'UniformOutput', false);
    else
        values = {};
    end
end
