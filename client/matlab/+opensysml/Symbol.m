classdef Symbol < handle
%SYMBOL One model element with lazily fetched members.

    properties (SetAccess = private)
        id = ''
        name = ''
        kind = ''
        metadata
        typeFacts = []
        multiplicity = []
        specializations = {}
        withheldLibraryAttributes = 0
        record = struct()
        model
    end

    properties (Access = private)
        childrenCache = []
        attributesCache = []
        childrenLoaded = false
        attributesLoaded = false
    end

    methods
        function obj = Symbol(record, model)
            if nargin < 1, record = struct(); end
            if nargin < 2, model = []; end
            obj.record = record;
            obj.model = model;
            obj.id = fieldOr(record, 'id', '');
            obj.name = fieldOr(record, 'name', '');
            obj.kind = fieldOr(record, 'kind', '');
            obj.metadata = metadataMap(fieldOr(record, 'metadata', struct()));
            if isfield(record, 'typeInfo') && ~isempty(fieldnames(record.typeInfo))
                obj.typeFacts = record.typeInfo;
            end
            if isfield(record, 'multiplicity') && ~isempty(fieldnames(record.multiplicity))
                obj.multiplicity = multiplicityOf(record.multiplicity);
            end
            obj.specializations = toCells(fieldOr(record, 'specializations', {}));
            obj.withheldLibraryAttributes = fieldOr(record, 'withheldLibraryAttributes', 0);
        end

        function values = children(obj)
            if obj.childrenLoaded, values = obj.childrenCache; return; end
            ids = textCells(fieldOr(obj.record, 'childIds', {}));
            values = {};
            if isempty(obj.model)
                obj.childrenCache = values;
                obj.childrenLoaded = true;
                return;
            end
            for i = 1:numel(ids)
                child = fetchSymbol(obj.model, ids{i});
                if ~isempty(child), values{end+1} = child; end
            end
            obj.childrenCache = values;
            obj.childrenLoaded = true;
        end

        function values = attributes(obj)
            if obj.attributesLoaded, values = obj.attributesCache; return; end
            children = obj.children();
            declared = {};
            for i = 1:numel(children)
                if any(strcmpi(children{i}.kind, ...
                        {'AttributeDef','AttributeUsage','ReferenceUsage'}))
                    declared{end+1} = children{i};
                end
            end
            byName = containers.Map('KeyType', 'char', 'ValueType', 'any');
            for i = 1:numel(declared), byName(declared{i}.name) = declared{i}; end
            inherited = inheritedAttributes(obj, {});
            for i = 1:numel(inherited)
                if ~isKey(byName, inherited{i}.name)
                    byName(inherited{i}.name) = inherited{i};
                end
            end
            reported = {};
            raw = toCells(fieldOr(obj.record, 'attributes', {}));
            for i = 1:numel(raw)
                if isfield(raw{i}, 'name'), reported{end+1} = raw{i}.name; end
            end
            values = {};
            for i = 1:numel(reported)
                if isKey(byName, reported{i}), values{end+1} = byName(reported{i}); end
            end
            for i = 1:numel(declared)
                if ~any(strcmp(reported, declared{i}.name)), values{end+1} = declared{i}; end
            end
            obj.attributesCache = values;
            obj.attributesLoaded = true;
        end

        function values = parts(obj)
            children = obj.children();
            values = {};
            for i = 1:numel(children)
                if any(strcmpi(children{i}.kind, {'PartDef','PartUsage'}))
                    values{end+1} = children{i};
                end
            end
        end

        function value = getAttr(obj, name)
            value = [];
            attrs = obj.attributes();
            for i = 1:numel(attrs)
                if strcmp(attrs{i}.name, name), value = attrs{i}; return; end
            end
        end

        function facts = attributeFacts(obj)
            if ~isempty(obj.model), obj.model.connection.require('symbol_attributes'); end
            raw = toCells(fieldOr(obj.record, 'attributes', {}));
            facts = cell(1, numel(raw));
            for i = 1:numel(raw)
                fact = struct('name', fieldOr(raw{i}, 'name', ''), ...
                    'type', fieldOr(raw{i}, 'type', ''), ...
                    'value', [], 'unit', fieldOr(raw{i}, 'unit', ''));
                if isfield(raw{i}, 'value')
                    try, fact.value = opensysml.decodeValue(raw{i}.value);
                    catch e, fact.value = e;
                    end
                end
                facts{i} = fact;
            end
        end

        function value = facts(obj)
            value = struct('id', obj.id, 'name', obj.name, 'kind', obj.kind, ...
                'type', obj.typeFacts, 'multiplicity', obj.multiplicity, ...
                'specializations', {obj.specializations}, ...
                'attributes', {obj.attributeFacts()}, ...
                'withheldLibraryAttributes', obj.withheldLibraryAttributes);
        end
    end
end

function symbols = inheritedAttributes(obj, visited)
    symbols = {};
    if isempty(obj.model) || any(strcmp(visited, obj.id)), return; end
    visited{end+1} = obj.id;
    for i = 1:numel(obj.specializations)
        spec = obj.specializations{i};
        target = fieldOr(spec, 'targetId', '');
        if isempty(target) || any(strcmp(visited, target)), continue; end
        super = fetchSymbol(obj.model, target);
        if isempty(super), continue; end
        children = super.children();
        for j = 1:numel(children)
            if any(strcmpi(children{j}.kind, ...
                    {'AttributeDef','AttributeUsage','ReferenceUsage'}))
                symbols{end+1} = children{j};
            end
        end
        symbols = [symbols inheritedAttributes(super, visited)];
    end
end

function symbol = fetchSymbol(model, id)
    symbol = [];
    try
        raw = opensysml.symbol(model, id);
        symbol = opensysml.Symbol(raw, model);
    catch e
        if ~strcmp(e.identifier, 'opensysml:connect:symbolNotFound'), rethrow(e); end
    end
end

function map = metadataMap(raw)
    if isa(raw, 'containers.Map'), map = raw; return; end
    map = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isstruct(raw)
        names = fieldnames(raw);
        for i = 1:numel(names), map(names{i}) = raw.(names{i}); end
    end
end

function multiplicity = multiplicityOf(raw)
    lower = fieldOr(raw, 'lower', '');
    upper = fieldOr(raw, 'upper', '');
    isCollection = [];
    if ~isempty(upper)
        if strcmp(upper, '*'), isCollection = true;
        else
            parsed = str2double(upper);
            if ~isnan(parsed), isCollection = parsed > 1; end
        end
    end
    isOptional = [];
    if ~isempty(lower)
        parsed = str2double(lower);
        if ~isnan(parsed), isOptional = parsed == 0; end
    end
    multiplicity = struct('lower', lower, 'upper', upper, ...
        'isCollection', isCollection, 'isOptional', isOptional);
end

function values = textCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    else, values = cellstr(raw(:))';
    end
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {raw};
    end
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end
