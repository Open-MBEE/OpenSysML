classdef CalcResult
%CALCRESULT What a calculation computed.

    properties
        value = []
        outputs = struct()
        diagnostics = {}
        standing
        engine = ''
        strength = ''
        bounds = {}
    end

    methods
        function obj = CalcResult(value, outputs, diagnostics, standing)
            if nargin >= 1, obj.value = value; end
            if nargin >= 2 && ~isempty(outputs), obj.outputs = outputs; end
            if nargin >= 3, obj.diagnostics = toCells(diagnostics); end
            if nargin >= 4 && ~isempty(standing), obj.standing = standing;
            else, obj.standing = opensysml.Standing();
            end
            obj.engine = obj.standing.engine;
            obj.strength = obj.standing.strength;
            obj.bounds = obj.standing.bounds;
        end

        function text = char(obj)
            [names, values] = opensysml.internal.mapEntries(obj.outputs);
            if isempty(names)
                text = valueText(obj.value);
            else
                lines = cell(1, numel(names));
                for i = 1:numel(names)
                    lines{i} = sprintf('%s = %s', names{i}, valueText(values{i}));
                end
                text = strjoin(lines, ', ');
            end
        end
    end
end

function cells = toCells(raw)
    if isempty(raw), cells = {};
    elseif iscell(raw), cells = raw(:)';
    elseif isstruct(raw), cells = num2cell(raw(:)');
    else, cells = {raw};
    end
end

function text = valueText(value)
    if ischar(value), text = value;
    elseif isobject(value) && ismethod(value, 'char'), text = char(value);
    elseif isnumeric(value) || islogical(value), text = num2str(value);
    else, text = evalc('disp(value)'); text = strtrim(text);
    end
end
