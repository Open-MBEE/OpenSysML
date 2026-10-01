function model = parseFile(conn, path, varargin)
%PARSEFILE Parse a file the service's process can read.
%   opensysml.parseFile(conn, path, 'language', 'sysml', 'strict', true)

    language = ''; strict = false;
    for i = 1:2:numel(varargin)
        switch varargin{i}
            case 'language', language = varargin{i+1};
            case 'strict', strict = varargin{i+1};
        end
    end
    request.filePath = char(path);
    if ~isempty(language), request.language = language; end
    if strict, request.strictConformance = true; end
    answer = opensysml.internal.checkError(opensysml.call(conn, 'ParseFile', request), 'ParseFile');
    diags = {};
    if isfield(answer, 'diagnostics') && ~isempty(answer.diagnostics)
        raw = answer.diagnostics;
        if isstruct(raw), raw = num2cell(raw); end
        diags = cellfun(@opensysml.internal.decodeDiagnostic, raw, 'UniformOutput', false);
    end
    model = opensysml.Model(conn, answer.modelHash, diags);
end
