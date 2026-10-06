classdef Model < handle
%MODEL A parsed model with roots, diagnostics and lazy symbol lookup.

    properties
        connection
        hash = ''
        diagnostics = {}
        roots = {}
        root = []
        documents = {}
        sourcePath = ''
    end

    methods
        function m = Model(conn, hash, diags, roots, documents, sourcePath)
            if nargin < 1, conn = []; end
            if nargin < 2, hash = ''; end
            if nargin < 3, diags = {}; end
            if nargin < 4, roots = {}; end
            if nargin < 5, documents = {}; end
            if nargin < 6, sourcePath = ''; end
            m.connection = conn;
            m.hash = hash;
            m.diagnostics = toCells(diags);
            m.documents = textCells(documents);
            m.sourcePath = sourcePath;
            rawRoots = toCells(roots);
            m.roots = cell(1, numel(rawRoots));
            for i = 1:numel(rawRoots)
                if isa(rawRoots{i}, 'opensysml.Symbol')
                    m.roots{i} = rawRoots{i};
                else
                    m.roots{i} = opensysml.Symbol(rawRoots{i}, m);
                end
            end
            if ~isempty(m.roots), m.root = m.roots{1}; end
        end

        function values = errors(m)
            values = {};
            for i = 1:numel(m.diagnostics)
                if strcmpi(m.diagnostics{i}.severity, 'error')
                    values{end+1} = m.diagnostics{i};
                end
            end
        end

        function tf = ok(m)
            tf = isempty(m.errors());
        end

        function editor = edit(m)
            editor = opensysml.Editor(m);
        end

        function result = raiseForErrors(m)
            errors = m.errors();
            if isempty(errors), result = m; return; end
            parts = cell(1, min(numel(errors), 3));
            for i = 1:numel(parts), parts{i} = diagnosticText(errors{i}); end
            summary = strjoin(parts, '; ');
            if numel(errors) > 3
                summary = sprintf('%s; ... and %d more', summary, numel(errors) - 3);
            end
            where = m.sourcePath;
            if isempty(where), where = 'the model'; end
            opensysml.internal.raise('opensysml:diagnostics:model', ...
                sprintf('%s has %d error(s): %s', where, numel(errors), summary), ...
                errors, struct('model', m));
            result = m;
        end

        function conversion = convert(m, toFormat, varargin)
            tolerate = false;
            for i = 1:2:numel(varargin)
                if i == numel(varargin)
                    opensysml.internal.raise('opensysml:argument', ...
                        'Model.convert options must be name-value pairs');
                end
                if strcmp(varargin{i}, 'tolerateSyntaxErrors')
                    tolerate = varargin{i+1};
                else
                    opensysml.internal.raise('opensysml:argument', ...
                        sprintf('Model.convert has no option ''%s''', varargin{i}));
                end
            end
            conversion = opensysml.convert(m.connection, toFormat, ...
                'modelHash', m.hash, 'fromFormat', 'sysml', ...
                'tolerateSyntaxErrors', tolerate);
        end

        function conversion = toSysml(m, varargin)
            conversion = m.convert('sysml', varargin{:});
        end

        function conversion = toTurtle(m)
            conversion = m.convert('ttl');
        end

        function conversion = toApiJson(m)
            conversion = m.convert('api-json');
        end

        function conversion = save(m, path, varargin)
            toFormat = '';
            tolerate = false;
            for i = 1:2:numel(varargin)
                if i == numel(varargin)
                    opensysml.internal.raise('opensysml:argument', ...
                        'Model.save options must be name-value pairs');
                end
                switch varargin{i}
                    case 'format', toFormat = char(varargin{i+1});
                    case 'tolerateSyntaxErrors', tolerate = varargin{i+1};
                    otherwise
                        opensysml.internal.raise('opensysml:argument', ...
                            sprintf('Model.save has no option ''%s''', varargin{i}));
                end
            end
            if isempty(toFormat), toFormat = opensysml.formatOfPath(path); end
            conversion = m.convert(toFormat, 'tolerateSyntaxErrors', tolerate);
            conversion.write(path);
        end

        function results = query(m, varargin)
            if isempty(varargin)
                payload = struct('type', 'Query');
                results = opensysml.query(m, payload);
            else
                results = opensysml.query(m, varargin{:});
            end
        end

        function result = runDocumentQuery(m, queryId, varargin)
            result = opensysml.runDocumentQuery(m, queryId, varargin{:});
        end

        function text = renderDocument(m, documentId, varargin)
            text = opensysml.renderDocument(m, documentId, varargin{:});
        end

        function result = renderView(m, viewName, varargin)
            result = opensysml.renderView(m, viewName, varargin{:});
        end

        function result = symbol(m, id)
            result = opensysml.Symbol(opensysml.getSymbol(m, id), m);
        end

        function result = get(m, fqn)
            result = findById(m, char(fqn));
        end

        function result = find(m, name)
            name = char(name);
            result = [];
            for i = 1:numel(m.roots)
                if strcmp(m.roots{i}.name, name) || strcmp(m.roots{i}.id, name)
                    result = m.roots{i};
                    return;
                end
            end
            if ~isempty(strfind(name, '::'))
                result = findById(m, name);
                if ~isempty(result), return; end
                result = symbolNamed(m, name);
            else
                result = symbolNamed(m, name);
                if isempty(result), result = findById(m, name); end
            end
        end

        function value = evaluate(m, expression, varargin)
            value = opensysml.evaluate(m, expression, varargin{:});
        end

        function value = instantiate(m, typeId)
            value = opensysml.instantiate(m, typeId);
        end

        function value = executeAction(m, actionId, varargin)
            value = opensysml.executeAction(m, actionId, varargin{:});
        end

        function value = exploreAction(m, actionId, varargin)
            value = opensysml.exploreAction(m, actionId, varargin{:});
        end

        function value = executeState(m, stateId, varargin)
            value = opensysml.executeState(m, stateId, varargin{:});
        end

        function value = exploreState(m, stateId, varargin)
            value = opensysml.exploreState(m, stateId, varargin{:});
        end

        function value = verifyConstraint(m, symbolId, varargin)
            value = opensysml.verifyConstraint(m, symbolId, varargin{:});
        end

        function value = verifyRequirement(m, symbolId, varargin)
            value = opensysml.verifyRequirement(m, symbolId, varargin{:});
        end

        function values = verifySatisfaction(m, varargin)
            values = opensysml.verifySatisfaction(m, varargin{:});
        end

        function tf = satisfied(m, symbolId)
            if nargin < 2, values = m.verifySatisfaction();
            else, values = m.verifySatisfaction('symbol', symbolId);
            end
            tf = true;
            for i = 1:numel(values), tf = tf && values{i}.holds; end
        end

        function value = validateInstance(m, symbolId, varargin)
            value = opensysml.validateInstance(m, symbolId, varargin{:});
        end

        function value = calc(m, symbolId, varargin)
            value = opensysml.calc(m, symbolId, varargin{:});
        end

        function value = runAnalysis(m, symbolId, varargin)
            value = opensysml.runAnalysis(m, symbolId, varargin{:});
        end

        function value = exploreAnalysis(m, symbolId, varargin)
            value = opensysml.exploreAnalysis(m, symbolId, varargin{:});
        end

        function value = runSweep(m, symbolId, ranges, varargin)
            value = opensysml.runSweep(m, symbolId, ranges, varargin{:});
        end

        function values = walk(m, depth)
            if nargin < 2, depth = Inf; end
            if ~isscalar(depth) || ~isnumeric(depth) || depth < 0
                opensysml.internal.raise('opensysml:argument', 'walk depth must be a non-negative number');
            end
            values = {};
            queue = cell(1, numel(m.roots));
            levels = zeros(1, numel(m.roots));
            for i = 1:numel(m.roots), queue{i} = m.roots{i}; end
            while ~isempty(queue)
                current = queue{1};
                level = levels(1);
                queue(1) = [];
                levels(1) = [];
                if level >= depth, continue; end
                children = current.children();
                for i = 1:numel(children)
                    values{end+1} = children{i};
                    queue{end+1} = children{i};
                    levels(end+1) = level + 1;
                end
            end
        end
    end
end

function result = symbolNamed(model, name)
    result = [];
    useQuery = false;
    if ~isempty(model.connection)
        useQuery = model.connection.hasCapability('query');
    end
    if ~useQuery
        result = walkNamed(model, name);
        return;
    end
    matches = queryOwner(model, 'name', name);
    candidateIds = cellfun(@(item) item.id, matches, 'UniformOutput', false);
    candidateDepths = [];
    if numel(candidateIds) > 1 || (~isempty(candidateIds) && ~model.ok())
        candidateDepths = ownerDepths(model, matches);
        [~, order] = sortrows([candidateDepths(:), (1:numel(candidateIds))']);
        candidateIds = candidateIds(order(:)');
        candidateDepths = candidateDepths(order(:)');
    end
    if ~model.ok()
        depth = Inf;
        if ~isempty(candidateIds), depth = candidateDepths(1); end
        result = walkNamed(model, name, depth);
        if ~isempty(result), return; end
    end
    for i = 1:numel(candidateIds)
        result = findById(model, candidateIds{i});
        if ~isempty(result), return; end
    end
end

function matches = queryOwner(model, property, value)
    payload = struct('type', 'PrimitiveConstraint', 'operator', '=', ...
        'property', property, 'value', {value});
    matches = opensysml.query(model, 'where', payload, 'select', {'owner'});
end

function depths = ownerDepths(model, elements)
    owners = containers.Map('KeyType', 'char', 'ValueType', 'char');
    rootIds = cell(1, numel(model.roots));
    for i = 1:numel(model.roots)
        rootIds{i} = model.roots{i}.id;
        owners(rootIds{i}) = '';
    end
    for i = 1:numel(elements)
        owners(elements{i}.id) = queryProperty(elements{i}, 'owner');
    end
    unknown = unknownOwners(owners);
    while ~isempty(unknown)
        parents = queryOwner(model, '@id', sort(unknown));
        for i = 1:numel(parents)
            owners(parents{i}.id) = queryProperty(parents{i}, 'owner');
        end
        for i = 1:numel(unknown)
            if ~isKey(owners, unknown{i}), owners(unknown{i}) = ''; end
        end
        unknown = unknownOwners(owners);
    end
    depths = zeros(1, numel(elements));
    for i = 1:numel(elements)
        id = elements{i}.id;
        hops = 0;
        while isKey(owners, id) && ~isempty(owners(id))
            id = owners(id);
            hops = hops + 1;
        end
        if ~any(strcmp(rootIds, id)), hops = hops + 1; end
        depths(i) = hops;
    end
end

function values = unknownOwners(owners)
    values = {};
    names = keys(owners);
    for i = 1:numel(names)
        parent = owners(names{i});
        if ~isempty(parent) && ~isKey(owners, parent)
            values{end+1} = parent;
        end
    end
end

function value = queryProperty(element, name)
    value = '';
    if isfield(element, 'properties') && isa(element.properties, 'containers.Map') && ...
            isKey(element.properties, name)
        raw = element.properties(name);
        if ischar(raw), value = raw;
        elseif iscell(raw) && numel(raw) == 1 && ischar(raw{1}), value = raw{1};
        end
    end
end

function result = walkNamed(model, name, depth)
    if nargin < 3, depth = Inf; end
    result = [];
    values = model.walk(depth);
    for i = 1:numel(values)
        if strcmp(values{i}.name, name), result = values{i}; return; end
    end
end

function result = findById(model, id)
    result = [];
    try
        raw = opensysml.call(model.connection, 'GetSymbol', ...
            struct('modelHash', model.hash, 'symbolId', id));
        if isfield(raw, 'error') && ~isempty(raw.error), return; end
        if isfield(raw, 'symbol'), raw = raw.symbol; end
        if isfield(raw, 'id') && strcmp(raw.id, id)
            result = opensysml.Symbol(raw, model);
        end
    catch e
        if ~strcmp(e.identifier, 'opensysml:connect:symbolNotFound'), rethrow(e); end
    end
end

function text = diagnosticText(diagnostic)
    text = sprintf('%s:%d:%d: %s: %s', diagnostic.file, diagnostic.line, ...
        diagnostic.col, diagnostic.severity, diagnostic.message);
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {raw};
    end
end

function values = textCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    else, values = cellstr(raw(:))';
    end
end
