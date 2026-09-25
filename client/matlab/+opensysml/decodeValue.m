function val = decodeValue(v)
%DECODEVALUE Decode one wire Value: an object with exactly one of the
%nineteen arm keys. Accepts every shape jsondecode produces — a struct for
%one object, a struct array or cell array for a list of objects, [] for
%null or an empty answer. Non-empty `null` strings, `infinity:false`, an
%empty function.calcId and shape-mismatched array/tensor bodies are errors,
%never default values.

    if isempty(v)
        val = [];
        return;
    end
    kind = fieldnames(v);
    kind = kind{1};
    asInt64 = @opensysml.parseInt64;
    asReal = @realOf;
    switch kind
        case 'intValue',    val = asInt64(v.intValue);
        case 'realValue',   val = asReal(v.realValue);
        case 'boolValue',   val = logical(v.boolValue);
        case 'stringValue', val = char(v.stringValue);
        case 'instanceId',  val = struct('instanceRef', asInt64(v.instanceId));
        case 'sequence',    val = mapValues(v.sequence, 'elements');
        case 'null'
            if ~isempty(v.null)
                error('opensysml:unsupported', 'unsupported value: %s', v.null);
            end
            val = [];
        case 'unset',       val = struct('unset', true);
        case 'quantity',    val = decodeQuantity(v.quantity);
        case 'enumLiteral'
            l = v.enumLiteral;
            scalar = [];
            if isfield(l, 'value'), scalar = opensysml.decodeValue(l.value); end
            name = '';
            if isfield(l, 'name'), name = l.name; end
            enumId = '';
            if isfield(l, 'enumerationId'), enumId = l.enumerationId; end
            val = struct('literalId', l.literalId, 'enumerationId', enumId, ...
                         'name', name, 'value', scalar);
        case 'complex'
            re = 0; im = 0;
            if isfield(v.complex, 'real'), re = asReal(v.complex.real); end
            if isfield(v.complex, 'imaginary'), im = asReal(v.complex.imaginary); end
            val = complex(re, im);
        case 'array'
            a = v.array;
            dims = {};
            if isfield(a, 'dimensions'), dims = a.dimensions; end
            dims = asInt64List(dims);
            if any(dims <= 0)
                error('opensysml:decode', 'array dimension is not positive');
            end
            elements = mapValues(a, 'elements');
            if prod(double(dims)) ~= numel(elements)
                error('opensysml:decode', 'array has %d elements for its dimensions', numel(elements));
            end
            val = struct('dimensions', dims, 'elements', {elements});
        case 'vector'
            components = fieldList(v.vector, 'components');
            val = struct('components', {cellfun(@decodeNumeric, components, 'UniformOutput', false)});
        case 'vectorQuantity'
            components = fieldList(v.vectorQuantity, 'components');
            if isempty(components)
                error('opensysml:decode', 'vectorQuantity has no components');
            end
            val = struct('components', {cellfun(@decodeQuantity, components, 'UniformOutput', false)});
        case 'measurementRef'
            m = v.measurementRef;
            unit = ''; if isfield(m, 'unit'), unit = m.unit; end
            unitId = []; if isfield(m, 'unitId'), unitId = m.unitId; end
            if isempty(unit) && isempty(unitId)
                error('opensysml:decode', 'measurementRef carries neither unit nor unitId');
            end
            if (~isempty(unit) || ~isempty(unitId)) && ~isfield(m, 'unitTerm')
                error('opensysml:decode', 'measurementRef carries a unit without its unitTerm');
            end
            val = struct('unit', unit, 'unitId', unitId, 'unitTerm', m.unitTerm);
        case 'infinity'
            if ~isequal(v.infinity, true) && ~isequal(v.infinity, 1)
                error('opensysml:decode', 'infinity arm does not carry true');
            end
            val = struct('infinity', true);
        case 'function'
            f = v.function;
            calcId = '';
            if isfield(f, 'calcId'), calcId = f.calcId; end
            if isempty(calcId)
                error('opensysml:decode', 'function carries no calcId');
            end
            self = [];
            if isfield(f, 'selfId') && ~strcmp(char(f.selfId), '0')
                self = struct('instanceRef', asInt64(f.selfId));
            end
            val = struct('calcId', calcId, 'self', self);
        case 'set'
            elements = mapValues(v.set, 'elements');
            val = struct('set', {elements});
            % members are compared pairwise by identity of their wire form
            for i = 1:numel(elements)
                for j = i+1:numel(elements)
                    if isequaln(elements{i}, elements{j})
                        error('opensysml:decode', 'set lists a member more than once');
                    end
                end
            end
        case 'tensorQuantity'
            t = v.tensorQuantity;
            dims = {};
            if isfield(t, 'dimensions'), dims = t.dimensions; end
            dims = asInt64List(dims);
            if any(dims <= 0)
                error('opensysml:decode', 'tensorQuantity dimension is not positive');
            end
            components = fieldList(t, 'components');
            if prod(double(dims)) ~= numel(components)
                error('opensysml:decode', 'tensorQuantity has %d components for its dimensions', numel(components));
            end
            val = struct('dimensions', dims, ...
                         'components', {cellfun(@decodeQuantity, components, 'UniformOutput', false)});
        case 'metaobject'
            m = v.metaobject;
            elementId = '';
            if isfield(m, 'elementId'), elementId = m.elementId; end
            if isempty(elementId)
                error('opensysml:decode', 'metaobject carries no elementId');
            end
            metaclassId = '';
            if isfield(m, 'metaclassId'), metaclassId = m.metaclassId; end
            val = struct('elementId', elementId, 'metaclassId', metaclassId);
        case 'undetermined'
            u = v.undetermined;
            reason = ''; if isfield(u, 'reason'), reason = u.reason; end
            lower = ''; upper = '';
            if isfield(u, 'count')
                if isfield(u.count, 'lower'), lower = u.count.lower; end
                if isfield(u.count, 'upper'), upper = u.count.upper; end
            end
            val = struct('reason', reason, 'lower', lower, 'upper', upper);
        otherwise
            error('opensysml:unknownArm', 'unknown Value arm: %s', kind);
    end
end

function out = mapValues(body, field)
% map decodeValue over a Value list, accepting every shape jsondecode gives
    elements = {};
    if isfield(body, field)
        elements = fieldList(body, field);
    end
    out = cellfun(@opensysml.decodeValue, elements, 'UniformOutput', false);
end

function list = fieldList(body, field)
% jsondecode returns a struct array for a uniform object list, a cell for a
% mixed one, a numeric vector for a number list, [] for an absent one
    list = {};
    if numel(body) == 0 || ~isfield(body, field)
        return;
    end
    raw = body(1).(field);
    if isstruct(raw)
        list = num2cell(raw);
    elseif iscell(raw)
        list = raw;
    elseif ~isempty(raw)
        list = num2cell(raw);
    end
end

function n = decodeNumeric(c)
    if isfield(c, 'intValue')
        n = opensysml.parseInt64(c.intValue);
    elseif isfield(c, 'realValue')
        n = realOf(c.realValue);
    else
        error('opensysml:decode', 'vector component is not an intValue or realValue');
    end
end

function q = decodeQuantity(b)
    if isfield(b, 'intMagnitude')
        mag = opensysml.parseInt64(b.intMagnitude);
    elseif isfield(b, 'realMagnitude')
        mag = realOf(b.realMagnitude);
    else
        error('opensysml:decode', 'quantity carries neither intMagnitude nor realMagnitude');
    end
    unit = ''; if isfield(b, 'unit'), unit = b.unit; end
    term = [];
    if isfield(b, 'unitTerm'), term = b.unitTerm; end
    q = struct('magnitude', mag, 'unit', unit, 'unitTerm', term);
end

function dims = asInt64List(cells)
    if iscell(cells)
        dims = cellfun(@(s) opensysml.parseInt64(s), cells);
    else
        dims = opensysml.parseInt64(cells);
    end
end

function x = realOf(x)
% "NaN", "Infinity", "-Infinity" arrive as char; a whole double arrives
% without a fraction and is still a double here
    if ischar(x) || (exist('isstring', 'builtin') && isstring(x))
        switch char(x)
            case 'Infinity', x = Inf;
            case '-Infinity', x = -Inf;
            otherwise, x = NaN;
        end
    else
        x = double(x);
    end
end
