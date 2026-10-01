function model = parseSources(conn, documents, varargin)
%PARSESOURCES Parse related file and inline documents into one model.

    options = parseOptions(varargin{:});
    [sources, names, hasLanguage] = sourceDocuments(documents, options.language);
    capabilities = {'parse_sources'};
    conn.require('parse_sources');
    if hasLanguage
        conn.require('inline_language');
        capabilities{end+1} = 'inline_language';
    end
    if options.strictConformance
        conn.require('strict_conformance');
        capabilities{end+1} = 'strict_conformance';
    end
    request = struct('documents', {sources});
    if options.strictConformance, request.strictConformance = true; end
    answer = opensysml.call(conn, 'ParseSources', request, capabilities);
    opensysml.internal.checkError(answer, 'ParseSources');
    diagnostics = decodeDiagnostics(answer);
    roots = fieldOr(answer, 'roots', {});
    if isstruct(roots), roots = num2cell(roots(:)'); end
    if isempty(roots)
        opensysml.internal.raise('opensysml:connect:service', ...
            'ParseSources answered without root symbols');
    end
    model = opensysml.Model(conn, answer.modelHash, diagnostics, roots, names, '');
    if options.raiseForErrors, model.raiseForErrors(); end
end

function [sources, names, hasLanguage] = sourceDocuments(documents, defaultLanguage)
    if isa(documents, 'opensysml.SourceDocument')
        documents = {documents};
    elseif ischar(documents) || isStringValue(documents)
        documents = {documents};
    end
    if ~iscell(documents) || isempty(documents)
        opensysml.internal.raise('opensysml:argument', ...
            'parseSources needs a non-empty cell of paths, source pairs or SourceDocument objects');
    end
    sources = cell(1, numel(documents));
    names = cell(1, numel(documents));
    hasLanguage = false;
    for i = 1:numel(documents)
        item = documents{i};
        if isa(item, 'opensysml.SourceDocument')
            source = item;
        elseif ischar(item) || (isStringValue(item) && isscalar(item))
            source = opensysml.SourceDocument.file(item);
        elseif iscell(item) && numel(item) == 2
            source = opensysml.SourceDocument.inline(item{1}, item{2});
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('document %d is a path, a {name, content} pair or a SourceDocument', i));
        end
        name = source.documentName();
        if any(strcmp(names(1:i-1), name))
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('two documents share the name ''%s''', name));
        end
        names{i} = name;
        wire = struct();
        if ~isempty(source.path)
            wire.filePath = source.path;
        else
            wire.content = source.content;
            wire.name = source.name;
            language = source.language;
            if isempty(language), language = defaultLanguage; end
            if ischar(language)
                wire.language = language;
                hasLanguage = true;
            end
        end
        sources{i} = wire;
    end
end

function options = parseOptions(varargin)
    options = struct('language', [], 'strictConformance', false, 'raiseForErrors', false);
    strict = [];
    strictConformance = [];
    if mod(numel(varargin), 2) ~= 0
        opensysml.internal.raise('opensysml:argument', 'parseSources options must be name-value pairs');
    end
    for i = 1:2:numel(varargin)
        key = char(varargin{i});
        switch key
            case 'language'
                options.language = validateLanguage(varargin{i+1});
            case 'strict'
                strict = logicalOption(varargin{i+1}, 'strict');
            case 'strictConformance'
                strictConformance = logicalOption(varargin{i+1}, 'strictConformance');
            case 'raiseForErrors'
                options.raiseForErrors = logicalOption(varargin{i+1}, 'raiseForErrors');
            otherwise
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('parseSources has no option ''%s''', key));
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

function language = validateLanguage(language)
    if isempty(language), return; end
    if isStringValue(language) && isscalar(language), language = char(language); end
    if ~ischar(language) || ~any(strcmp(language, {'sysml', 'kerml'}))
        opensysml.internal.raise('opensysml:argument', ...
            'language must be ''sysml'' or ''kerml''');
    end
end

function value = logicalOption(raw, name)
    if ~isscalar(raw) || ~(islogical(raw) || isnumeric(raw))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a scalar logical value', name));
    end
    value = logical(raw);
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
