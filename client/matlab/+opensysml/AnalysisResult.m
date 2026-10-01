classdef AnalysisResult
%ANALYSISRESULT What an analysis case computed and decided.

    properties
        outputs = struct()
        verdicts = {}
        instances
        diagnostics = {}
        verifications = {}
        evaluations = {}
        standing
        engine = ''
        strength = ''
        bounds = {}
    end

    methods
        function obj = AnalysisResult(outputs, verdicts, instances, diagnostics, ...
                verifications, evaluations, standing)
            if nargin >= 1 && ~isempty(outputs), obj.outputs = outputs; end
            if nargin >= 2, obj.verdicts = toCells(verdicts); end
            if nargin >= 3, obj.instances = instanceMap(instances);
            else, obj.instances = instanceMap([]);
            end
            if nargin >= 4, obj.diagnostics = toCells(diagnostics); end
            if nargin >= 5, obj.verifications = toCells(verifications); end
            if nargin >= 6, obj.evaluations = toCells(evaluations); end
            if nargin >= 7 && ~isempty(standing), obj.standing = standing;
            else, obj.standing = opensysml.Standing();
            end
            obj.engine = obj.standing.engine;
            obj.strength = obj.standing.strength;
            obj.bounds = obj.standing.bounds;
        end

        function values = selected(obj)
            values = {};
            for i = 1:numel(obj.evaluations)
                if isfield(obj.evaluations{i}, 'selected') && obj.evaluations{i}.selected
                    values{end+1} = obj.evaluations{i};
                end
            end
        end

        function tf = satisfied(obj)
            tf = true;
            for i = 1:numel(obj.verdicts), tf = tf && obj.verdicts{i}.holds; end
        end

        function text = explain(obj)
            lines = {};
            names = fieldnames(obj.outputs);
            for i = 1:numel(names)
                lines{end+1} = sprintf('%s = %s', names{i}, valueText(obj.outputs.(names{i})));
            end
            for i = 1:numel(obj.verdicts), lines{end+1} = obj.verdicts{i}.explain(); end
            for i = 1:numel(obj.verifications)
                lines{end+1} = obj.verifications{i}.explanation;
            end
            for i = 1:numel(obj.evaluations)
                lines{end+1} = evaluationText(obj.evaluations{i});
            end
            text = strjoin(lines, sprintf('\n'));
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

function map = instanceMap(raw)
    if isa(raw, 'containers.Map'), map = raw; return; end
    map = containers.Map('KeyType', 'char', 'ValueType', 'any');
    if isstruct(raw)
        fields = fieldnames(raw);
        for i = 1:numel(fields), map(fields{i}) = raw.(fields{i}); end
    end
end

function text = valueText(value)
    if ischar(value), text = value;
    elseif isobject(value) && ismethod(value, 'char'), text = char(value);
    elseif isnumeric(value) || islogical(value), text = num2str(value);
    else, text = evalc('disp(value)'); text = strtrim(text);
    end
end

function text = evaluationText(evaluation)
    call = sprintf('%s(%s)', evaluation.functionId, ...
        strjoin(cellfun(@valueText, evaluation.arguments, 'UniformOutput', false), ', '));
    if ~isempty(evaluation.error), text = sprintf('%s: error: %s', call, evaluation.error);
    else, text = sprintf('%s = %s', call, valueText(evaluation.result));
    end
    if evaluation.selected, text = [text ' [selected]'];
    elseif evaluation.tied, text = [text ' [tied]'];
    end
end
