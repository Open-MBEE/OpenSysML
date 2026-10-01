classdef Verdict
%VERDICT One verification's answer.

    properties
        kind = ''
        elementId = ''
        element = ''
        holds = false
        condition = ''
        instanceId = int64(0)
        instanceTypeId = ''
        requirementId = ''
        instancePath = ''
        question = ''
        status = ''
        witness = {}
        error = ''
        standing
        engine = ''
        strength = ''
        bounds = {}
        instances
        diagnostics = {}
        verifications = {}
    end

    methods
        function obj = Verdict(raw, instances, diagnostics, verifications)
            if nargin < 1, raw = struct(); end
            if nargin < 2, instances = []; end
            if nargin < 3, diagnostics = {}; end
            if nargin < 4, verifications = {}; end
            obj.kind = textField(raw, 'kind', '');
            obj.elementId = textField(raw, 'elementId', '');
            obj.element = textField(raw, 'element', obj.elementId);
            obj.holds = logicalField(raw, 'holds', false);
            obj.condition = textField(raw, 'condition', '');
            obj.instanceId = intField(raw, 'instanceId', int64(0));
            obj.instanceTypeId = textField(raw, 'instanceTypeId', '');
            obj.requirementId = textField(raw, 'requirementId', '');
            obj.instancePath = textField(raw, 'instancePath', '');
            obj.question = textField(raw, 'question', '');
            obj.status = textField(raw, 'status', '');
            obj.error = textField(raw, 'error', '');
            obj.witness = witnessList(fieldOr(raw, 'witness', {}), instances);
            obj.standing = standingOf(raw);
            obj.engine = obj.standing.engine;
            obj.strength = obj.standing.strength;
            obj.bounds = obj.standing.bounds;
            obj.instances = instanceMap(instances);
            obj.diagnostics = toCells(diagnostics);
            obj.verifications = toCells(verifications);
        end

        function tf = evaluated(obj)
            tf = isempty(obj.error);
        end

        function result = raiseForError(obj)
            if ~isempty(obj.error)
                named = verdictName(obj);
                opensysml.internal.raise('opensysml:diagnostics:execution', ...
                    sprintf('%s: %s', named, obj.error), obj.diagnostics, ...
                    struct('failure', '', 'result', []));
            end
            result = obj;
        end

        function text = explain(obj)
            subject = '';
            if obj.instanceId ~= 0
                instanceType = obj.instanceTypeId;
                if isempty(instanceType), instanceType = 'instance'; end
                subject = sprintf(' (on %s ID: %d)', instanceType, obj.instanceId);
            end
            named = verdictName(obj);
            if ~isempty(obj.error)
                text = sprintf('? %s%s: %s', named, subject, obj.error);
            elseif obj.holds
                text = sprintf('%s %s holds%s', ...
                    opensysml.internal.unicodeChar(10003), named, subject);
            elseif isempty(obj.condition)
                text = sprintf('%s %s fails%s: condition evaluated to false', ...
                    opensysml.internal.unicodeChar(10007), named, subject);
            else
                text = sprintf('%s %s fails%s: condition evaluated to false: %s', ...
                    opensysml.internal.unicodeChar(10007), named, subject, obj.condition);
            end
            if obj.standing.reported()
                text = [text ' ' opensysml.internal.unicodeChar(8212) ...
                    ' ' obj.standing.explain()];
            end
        end

        function tf = logical(obj)
            tf = obj.holds;
        end

        function text = char(obj)
            text = obj.explain();
        end
    end
end

function text = verdictName(obj)
    element = obj.element;
    if isempty(element), element = obj.elementId; end
    tokens = regexp(element, '\S+', 'match');
    if ~any(strcmp(tokens, obj.kind))
        text = [obj.kind ' ' element];
    else
        text = element;
    end
    if ~isempty(obj.instancePath), text = [text ' at ' obj.instancePath]; end
end

function standing = standingOf(raw)
    bounds = toCells(fieldOr(raw, 'bounds', {}));
    for i = 1:numel(bounds)
        if isfield(bounds{i}, 'limit'), bounds{i}.limit = opensysml.parseInt64(valueString(bounds{i}.limit)); end
    end
    standing = opensysml.Standing(textField(raw, 'engine', ''), ...
        textField(raw, 'strength', ''), bounds);
end

function values = witnessList(raw, instances)
    raw = toCells(raw);
    values = cell(1, numel(raw));
    for i = 1:numel(raw)
        entry = raw{i};
        value = struct('name', textField(entry, 'feature', textField(entry, 'name', '')), ...
            'value', [], 'unit', textField(entry, 'unit', ''), ...
            'exact', textField(entry, 'exact', ''));
        if isfield(entry, 'value')
            if isa(instances, 'containers.Map')
                value.value = opensysml.decodeValue(entry.value, ...
                    opensysml.internal.instanceResolver(instances));
            else
                value.value = opensysml.decodeValue(entry.value);
            end
        end
        values{i} = value;
    end
end

function map = instanceMap(raw)
    if isa(raw, 'containers.Map'), map = raw; return; end
    map = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isa(raw, 'struct')
        names = fieldnames(raw);
        for i = 1:numel(names), map(names{i}) = raw.(names{i}); end
    end
end

function cells = toCells(raw)
    if isempty(raw), cells = {};
    elseif iscell(raw), cells = raw(:)';
    elseif isstruct(raw), cells = num2cell(raw(:)');
    else, cells = {raw};
    end
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end

function value = textField(record, name, fallback)
    value = fieldOr(record, name, fallback);
    if isempty(value), value = fallback; end
    if ~ischar(value), value = char(value); end
end

function value = logicalField(record, name, fallback)
    value = fieldOr(record, name, fallback);
    if isempty(value), value = fallback; end
    value = logical(value);
end

function value = intField(record, name, fallback)
    raw = fieldOr(record, name, fallback);
    if ischar(raw), value = opensysml.parseInt64(raw);
    else, value = int64(raw);
    end
end

function text = valueString(value)
    if ischar(value), text = value; else, text = sprintf('%d', int64(value)); end
end
