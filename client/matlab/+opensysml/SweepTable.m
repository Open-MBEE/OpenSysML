classdef SweepTable
%SWEEPTABLE Every run of one parameter sweep.

    properties
        rows = {}
        parameters = {}
        sampled = false
        seed = uint64(0)
        instances
        diagnostics = {}
        standing
        engine = ''
        strength = ''
        bounds = {}
    end

    methods
        function obj = SweepTable(rows, parameters, sampled, seed, instances, diagnostics, standing)
            if nargin >= 1, obj.rows = toCells(rows); end
            if nargin >= 2, obj.parameters = toCells(parameters); end
            if nargin >= 3, obj.sampled = logical(sampled); end
            if nargin >= 4, obj.seed = opensysml.parseUint64(seed); end
            if nargin >= 5, obj.instances = instanceMap(instances);
            else, obj.instances = instanceMap([]);
            end
            if nargin >= 6, obj.diagnostics = toCells(diagnostics); end
            if nargin >= 7 && ~isempty(standing), obj.standing = standing;
            else, obj.standing = opensysml.Standing();
            end
            obj.engine = obj.standing.engine;
            obj.strength = obj.standing.strength;
            obj.bounds = obj.standing.bounds;
        end

        function values = failures(obj)
            values = {};
            for i = 1:numel(obj.rows)
                if isfield(obj.rows{i}, 'failed') && obj.rows{i}.failed
                    values{end+1} = obj.rows{i};
                end
            end
        end

        function n = length(obj)
            n = numel(obj.rows);
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
