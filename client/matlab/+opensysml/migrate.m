function migration = migrate(conn, toFormat, varargin)
%MIGRATE Migrate a SysML v1 model to SysML v2, accounting for every element.
%   migration = opensysml.migrate(conn, toFormat, 'filePath', path) migrates
%   a Cameo/MagicDraw .mdzip, a UML XMI .xmi or an Eclipse UML2 .uml export
%   the service can read; 'content', bytes carries the file inline and needs
%   'fromFormat' to say which of xmi, uml or mdzip it is. toFormat is sysml,
%   kerml, ttl, turtle or rdf.
%
%   A migration is ledgered, not lossless: every v1 element lands in
%   migration.report as mapped, approximated, unmapped or skipped. 'report'
%   asks for every element's verdict and the report text, 'results' for the
%   JSON index of the v1 tool's result snapshots; 'layoutPath' or
%   'layoutContent' places the diagrams by a Cameo MTIP layout, 'imageBaseUrl'
%   prefixes image references and 'strict' refuses probabilities that do not
%   sum to one. Migration is experimental and says so with the
%   opensysml:experimental warning.

    sources = struct('filePath', [], 'content', []);
    options = struct('fromFormat', '', 'report', false, 'results', false, ...
        'layoutPath', '', 'layoutContent', '', 'imageBaseUrl', '', 'strict', false);
    if mod(numel(varargin), 2) ~= 0
        opensysml.internal.raise('opensysml:argument', 'migrate options must be name-value pairs');
    end
    for i = 1:2:numel(varargin)
        key = char(varargin{i});
        if isfield(sources, key)
            sources.(key) = varargin{i+1};
        elseif isfield(options, key)
            options.(key) = varargin{i+1};
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('migrate has no option ''%s''', key));
        end
    end
    present = {};
    if ischar(sources.filePath) || isStringScalar(sources.filePath)
        present{end+1} = 'filePath';
    elseif ~isempty(sources.filePath)
        opensysml.internal.raise('opensysml:argument', 'filePath must be text');
    end
    if ischar(sources.content) || isStringScalar(sources.content) || isa(sources.content, 'uint8')
        present{end+1} = 'content';
    elseif ~isempty(sources.content)
        opensysml.internal.raise('opensysml:argument', 'content must be text or uint8 bytes');
    end
    if numel(present) ~= 1
        if isempty(present), given = 'none'; else, given = strjoin(present, ', '); end
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('Provide exactly one of filePath or content; got %s', given));
    end
    toFormat = textOption(toFormat, 'toFormat');
    fromFormat = textOption(options.fromFormat, 'fromFormat');
    layoutPath = textOption(options.layoutPath, 'layoutPath');
    layoutContent = textOption(options.layoutContent, 'layoutContent');
    imageBaseUrl = textOption(options.imageBaseUrl, 'imageBaseUrl');
    report = logicalOption(options.report, 'report');
    results = logicalOption(options.results, 'results');
    strict = logicalOption(options.strict, 'strict');
    if strcmp(present{1}, 'filePath'), name = char(sources.filePath); else, name = 'the source'; end
    if ~isempty(fromFormat) && ~opensysml.isV1(fromFormat)
        opensysml.internal.raise('opensysml:argument', sprintf(['%s is %s input, which is ' ...
            'converted, not migrated: only a SysML v1 model (xmi, uml or mdzip) is migrated; ' ...
            'call opensysml.convert with the same source'], name, fromFormat));
    end
    if strcmp(present{1}, 'content') && isempty(fromFormat)
        opensysml.internal.raise('opensysml:argument', ...
            'fromFormat is required for inline content: xmi, uml or mdzip');
    end
    if ~isempty(layoutPath) && ~isempty(layoutContent)
        opensysml.internal.raise('opensysml:argument', ...
            'Provide at most one of layoutPath or layoutContent');
    end
    conn.require('migrate');
    request = struct('toFormat', toFormat, 'fromFormat', fromFormat, 'report', report, ...
        'results', results, 'imageBaseUrl', imageBaseUrl, 'strict', strict);
    sourcePath = '';
    if strcmp(present{1}, 'filePath')
        request.filePath = char(sources.filePath);
        sourcePath = opensysml.internal.absolutePath(request.filePath);
    else
        content = sources.content;
        if isStringScalar(content), content = char(content); end
        if ischar(content), content = unicode2native(content, 'UTF-8'); end
        request.content = opensysml.internal.base64Encode(content);
    end
    if ~isempty(layoutPath), request.layoutPath = layoutPath; end
    if ~isempty(layoutContent), request.layoutContent = layoutContent; end
    answer = opensysml.internal.checkError( ...
        opensysml.call(conn, 'Migrate', request, {'migrate'}), 'Migrate');
    migration = opensysml.Migration(answer, fromFormat, toFormat, sourcePath);
end

function value = textOption(raw, name)
    if isStringScalar(raw), raw = char(raw); end
    if ~ischar(raw)
        opensysml.internal.raise('opensysml:argument', sprintf('%s must be text', name));
    end
    value = raw;
end

function value = logicalOption(raw, name)
    if ~isscalar(raw) || ~(islogical(raw) || isnumeric(raw))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a scalar logical value', name));
    end
    value = logical(raw);
end

function tf = isStringScalar(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value) && isscalar(value);
    end
end
