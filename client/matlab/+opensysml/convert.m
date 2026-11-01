function conversion = convert(conn, toFormat, varargin)
%CONVERT Convert a file, inline source or cached model.

    sources = struct('filePath', [], 'content', [], 'modelHash', []);
    options = struct('fromFormat', '', 'tolerateSyntaxErrors', false, 'idForm', '', ...
        'includeLibraryStubs', false, 'compact', false, 'omitDerived', false, ...
        'keepDerived', {{}}, 'includeCheckDiagnostics', false);
    if mod(numel(varargin), 2) ~= 0
        opensysml.internal.raise('opensysml:argument', 'convert options must be name-value pairs');
    end
    for i = 1:2:numel(varargin)
        key = char(varargin{i});
        if isfield(sources, key)
            sources.(key) = varargin{i+1};
        elseif isfield(options, key)
            options.(key) = varargin{i+1};
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('convert has no option ''%s''', key));
        end
    end
    present = {};
    names = {'filePath','content','modelHash'};
    for i = 1:numel(names)
        value = sources.(names{i});
        if ischar(value) || isStringScalar(value)
            present{end+1} = names{i};
        elseif ~isempty(value)
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s must be text', names{i}));
        end
    end
    if numel(present) ~= 1
        if isempty(present), given = 'none'; else, given = strjoin(present, ', '); end
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('Provide exactly one of filePath, content or modelHash; got %s', given));
    end
    toFormat = textOption(toFormat, 'toFormat');
    fromFormat = textOption(options.fromFormat, 'fromFormat');
    tolerate = logicalOption(options.tolerateSyntaxErrors, 'tolerateSyntaxErrors');
    idForm = textOption(options.idForm, 'idForm');
    includeLibraryStubs = logicalOption(options.includeLibraryStubs, 'includeLibraryStubs');
    compact = logicalOption(options.compact, 'compact');
    omitDerived = logicalOption(options.omitDerived, 'omitDerived');
    keepDerived = textListOption(options.keepDerived, 'keepDerived');
    includeCheckDiagnostics = logicalOption(options.includeCheckDiagnostics, 'includeCheckDiagnostics');
    v1File = strcmp(present{1}, 'filePath') && isempty(fromFormat) && ...
        opensysml.pathIsV1(sources.filePath);
    if opensysml.isV1(fromFormat) || v1File
        if strcmp(present{1}, 'filePath'), name = char(sources.filePath); else, name = 'the source'; end
        opensysml.internal.raise('opensysml:argument', sprintf(['%s is a SysML v1 model, ' ...
            'which is migrated, not converted: every element is mapped, approximated or left ' ...
            'unmapped and reported element by element; call opensysml.migrate with the same ' ...
            'source'], name));
    end
    conn.require('convert');
    capabilities = {'convert'};
    if includeLibraryStubs
        conn.require('convert_library_stubs');
        capabilities{end+1} = 'convert_library_stubs';
    end
    if compact || omitDerived || ~isempty(keepDerived)
        conn.require('convert_compact');
        capabilities{end+1} = 'convert_compact';
    end
    if includeCheckDiagnostics
        conn.require('convert_check_diagnostics');
        capabilities{end+1} = 'convert_check_diagnostics';
    end
    request = struct('toFormat', toFormat);
    for i = 1:numel(present)
        name = present{i};
        value = sources.(name);
        if isStringScalar(value), value = char(value); end
        wireName = name;
        if strcmp(name, 'modelHash'), wireName = 'modelHash'; end
        request.(wireName) = value;
    end
    if ~isempty(fromFormat), request.fromFormat = fromFormat; end
    if tolerate, request.tolerateSyntaxErrors = true; end
    if ~isempty(idForm), request.idForm = idForm; end
    if includeLibraryStubs, request.includeLibraryStubs = true; end
    if compact, request.compact = true; end
    if omitDerived, request.omitDerived = true; end
    if ~isempty(keepDerived), request.keepDerived = keepDerived; end
    if includeCheckDiagnostics, request.includeCheckDiagnostics = true; end
    answer = opensysml.internal.checkError( ...
        opensysml.call(conn, 'Convert', request, capabilities), 'Convert');
    diagnostics = {};
    raw = fieldOr(answer, 'diagnostics', {});
    if isstruct(raw), raw = num2cell(raw(:)'); end
    if ~isempty(raw)
        diagnostics = cellfun(@opensysml.internal.decodeDiagnostic, raw, 'UniformOutput', false);
    end
    notice = fieldOr(answer, 'experimentalNotice', '');
    references = {};
    rawReferences = fieldOr(answer, 'libraryReferences', {});
    if isstruct(rawReferences), rawReferences = num2cell(rawReferences(:)'); end
    for i = 1:numel(rawReferences)
        ref = rawReferences{i};
        references{end+1} = struct('id', fieldOr(ref, 'id', ''), ...
            'qualifiedName', fieldOr(ref, 'qualifiedName', ''), 'type', fieldOr(ref, 'type', '')); %#ok<AGROW>
    end
    conversion = opensysml.Conversion(fieldOr(answer, 'content', ''), ...
        fieldOr(answer, 'fromFormat', fromFormat), ...
        fieldOr(answer, 'toFormat', toFormat), diagnostics, notice, references);
end

function value = textOption(raw, name)
    if isStringScalar(raw), raw = char(raw); end
    if ~ischar(raw)
        opensysml.internal.raise('opensysml:argument', sprintf('%s must be text', name));
    end
    value = raw;
end

function value = textListOption(raw, name)
    if isa(raw, 'string'), raw = cellstr(raw); end
    if ischar(raw), raw = {raw}; end
    if ~iscell(raw) || ~all(cellfun(@(item) ischar(item) || isStringScalar(item), raw))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('convert %s must be a cell array of text', name));
    end
    value = cellfun(@char, raw(:)', 'UniformOutput', false);
end

function value = logicalOption(raw, name)
    if ~isscalar(raw) || ~(islogical(raw) || isnumeric(raw))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a scalar logical value', name));
    end
    value = logical(raw);
end

function value = fieldOr(record, name, fallback)
    if isfield(record, name), value = record.(name); else, value = fallback; end
end

function tf = isStringScalar(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value) && isscalar(value);
    end
end
