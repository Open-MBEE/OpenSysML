function d = decodeDiagnostic(raw)
%DECODEDIAGNOSTIC One wire Diagnostic as a flat struct.

    d.severity = ''; d.message = ''; d.file = '';
    d.line = 0; d.col = 0; d.code = '';
    if isfield(raw, 'severity'), d.severity = raw.severity; end
    if isfield(raw, 'message'), d.message = raw.message; end
    if isfield(raw, 'code'), d.code = raw.code; end
    if isfield(raw, 'span')
        if isfield(raw.span, 'file'), d.file = raw.span.file; end
        if isfield(raw.span, 'startLine'), d.line = raw.span.startLine; end
        if isfield(raw.span, 'startCol'), d.col = raw.span.startCol; end
    end
end
