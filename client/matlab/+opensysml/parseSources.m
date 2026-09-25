function model = parseSources(conn, documents, varargin)
%PARSESOURCES Parse a cell of {name, content} pairs as one model; a cell
%array is a request list, so jsonencode writes one object per entry.

    language = ''; strict = false;
    for i = 1:2:numel(varargin)
        switch varargin{i}
            case 'language', language = varargin{i+1};
            case 'strict', strict = varargin{i+1};
        end
    end
    docs = {};
    for i = 1:numel(documents)
        pair = documents{i};
        doc.name = char(pair{1});
        doc.content = char(pair{2});
        if ~isempty(language), doc.language = language; end
        docs{end+1} = doc;
    end
    request.documents = docs;
    if strict, request.strictConformance = true; end
    answer = opensysml.internal.checkError(opensysml.call(conn, 'ParseSources', request), 'ParseSources');
    diags = {};
    if isfield(answer, 'diagnostics') && ~isempty(answer.diagnostics)
        raw = answer.diagnostics;
        if isstruct(raw), raw = num2cell(raw); end
        diags = cellfun(@opensysml.internal.decodeDiagnostic, raw, 'UniformOutput', false);
    end
    model = opensysml.Model(conn, answer.modelHash, diags);
end
