classdef Standing
%STANDING How strongly an engine answered, and within what bounds.

    properties
        engine = ''
        strength = ''
        bounds = {}
    end

    methods
        function obj = Standing(engine, strength, bounds)
            if nargin >= 1, obj.engine = engine; end
            if nargin >= 2, obj.strength = strength; end
            if nargin >= 3, obj.bounds = toCells(bounds); end
        end

        function tf = reported(obj)
            tf = ~isempty(obj.strength);
        end

        function bounds = reached(obj)
            bounds = {};
            for i = 1:numel(obj.bounds)
                if isfield(obj.bounds{i}, 'reached') && obj.bounds{i}.reached
                    bounds{end+1} = obj.bounds{i};
                end
            end
        end

        function text = explain(obj)
            if ~obj.reported(), text = ''; return; end
            text = obj.strength;
            if ~isempty(obj.engine), text = [text ' by ' obj.engine]; end
            reachedBounds = obj.reached();
            if ~isempty(reachedBounds)
                labels = cellfun(@boundText, reachedBounds, 'UniformOutput', false);
                text = [text ' (' strjoin(labels, ', ') ')'];
            end
        end

        function text = char(obj)
            text = obj.explain();
        end
    end
end

function items = toCells(raw)
    if isempty(raw), items = {}; return; end
    if iscell(raw), items = raw(:)'; else, items = num2cell(raw(:)'); end
end

function text = boundText(bound)
    text = sprintf('%s %s', bound.name, valueText(bound.limit));
    if isfield(bound, 'reached') && bound.reached, text = [text ' reached']; end
end

function text = valueText(value)
    if isa(value, 'int64'), text = sprintf('%d', value);
    elseif ischar(value), text = value;
    else, text = num2str(value);
    end
end
