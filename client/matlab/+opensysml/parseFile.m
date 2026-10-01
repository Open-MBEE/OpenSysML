function model = parseFile(conn, path, varargin)
%PARSEFILE Parse a file the service can read.

    options = parseOptions(varargin{:});
    capabilities = {};
    request = struct('filePath', char(path));
    if ~isempty(options.language)
        validateLanguage(options.language);
        request.language = char(options.language);
    end
    if options.strictConformance
        conn.require('strict_conformance');
        capabilities{end+1} = 'strict_conformance';
        request.strictConformance = true;
    end
    answer = opensysml.call(conn, 'ParseFile', request, capabilities);
    opensysml.internal.checkError(answer, 'ParseFile');
    diagnostics = decodeDiagnostics(answer);
    root = fieldOr(answer, 'root', struct());
    if isempty(fieldnames(root))
        opensysml.internal.raise('opensysml:connect:service', ...
            'ParseFile answered without a root symbol');
    end
    model = opensysml.Model(conn, answer.modelHash, diagnostics, ...
        {root}, {char(path)}, char(path));
    if options.raiseForErrors, model.raiseForErrors(); end
end

function options = parseOptions(varargin)
    options = struct('language', [], 'strictConformance', false, 'raiseForErrors', false);
    strict = [];
    strictConformance = [];
    if mod(numel(varargin), 2) ~= 0
        opensysml.internal.raise('opensysml:argument', 'parseFile options must be name-value pairs');
    end
    for i = 1:2:numel(varargin)
        key = char(varargin{i});
        switch key
            case 'language'
                options.language = varargin{i+1};
            case 'strict'
                strict = logicalOption(varargin{i+1}, 'strict');
            case 'strictConformance'
                strictConformance = logicalOption(varargin{i+1}, 'strictConformance');
            case 'raiseForErrors'
                options.raiseForErrors = logicalOption(varargin{i+1}, 'raiseForErrors');
            otherwise
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('parseFile has no option ''%s''', key));
        end
    end
    if ~isempty(strict) && ~isempty(strictConformance) && strict ~= strictConformance
        opensysml.internal.raise('opensysml:argument', ...
            'strict and strictConformance must agree when both are given');
    end
    if ~isempty(strict), options.strictConformance = strict;
    elseif ~isempty(strictConformance), options.strictConformance = strictConformance;
    end
end

function value = logicalOption(raw, name)
    if ~isscalar(raw) || ~(islogical(raw) || isnumeric(raw))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a scalar logical value', name));
    end
    value = logical(raw);
end

function validateLanguage(language)
    if isStringValue(language) && isscalar(language), language = char(language); end
    if ~ischar(language) || ~any(strcmp(language, {'sysml','kerml'}))
        opensysml.internal.raise('opensysml:argument', ...
            'language must be ''sysml'' or ''kerml''');
    end
end

function diagnostics = decodeDiagnostics(answer)
    diagnostics = {};
    raw = fieldOr(answer, 'diagnostics', {});
    if isempty(raw), return; end
    if isstruct(raw), raw = num2cell(raw(:)'); end
    diagnostics = cellfun(@opensysml.internal.decodeDiagnostic, raw, 'UniformOutput', false);
end

function value = fieldOr(record, name, fallback)
    if isfield(record, name), value = record.(name); else, value = fallback; end
end

function tf = isStringValue(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file'), tf = isstring(value); end
end
