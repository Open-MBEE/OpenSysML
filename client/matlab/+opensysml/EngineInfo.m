classdef EngineInfo
%ENGINEINFO One analysis engine registered by the service.

    properties
        name = ''
        authority = ''
        answers = {}
        bounds = {}
        process = ''
        processFound = ''
        ready = true
        unavailable = ''
        kind = ''
        protocol = ''
        source = ''
        command = ''
        version = ''
        served = false
    end

    methods
        function obj = EngineInfo(raw)
            if nargin < 1, raw = struct(); end
            names = {'name','authority','process','processFound','unavailable', ...
                'kind','protocol','source','command','version'};
            for i = 1:numel(names)
                key = names{i};
                if isfield(raw, key), obj.(key) = raw.(key); end
            end
            obj.answers = asCells(fieldOr(raw, 'answers', {}));
            obj.bounds = asCells(fieldOr(raw, 'bounds', {}));
            obj.ready = logical(fieldOr(raw, 'ready', true));
            obj.served = logical(fieldOr(raw, 'served', false));
        end

        function text = explain(obj)
            if obj.ready, status = 'ready';
            else, status = ['unavailable: ' obj.unavailable];
            end
            if strcmp(obj.kind, 'engine') && ~obj.served
                status = [status '; not served by this service'];
            end
            kind = '';
            if any(strcmp(obj.kind, {'engine', 'tool'}))
                kind = sprintf(' (%s, %s)', obj.kind, obj.protocol);
            end
            text = sprintf('%s%s: %s, answers %s; %s', obj.name, kind, ...
                obj.authority, strjoin(obj.answers, ', '), status);
        end

        function text = char(obj)
            text = obj.explain();
        end
    end
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end

function cells = asCells(raw)
    if isempty(raw), cells = {};
    elseif iscell(raw), cells = raw(:)';
    elseif isstruct(raw), cells = num2cell(raw(:)');
    else, cells = cellstr(raw);
    end
end
