classdef Body < handle
%BODY Chainable action-body statements for Editor sequence operations.

    properties (Access = private)
        itemList = {}
    end

    methods
        function obj = Body()
        end

        function items = operations(obj)
            items = obj.itemList;
            opensysml.internal.validateSequenceDepth(items);
        end

        function obj = addFirst(obj, ref)
            ref = opensysml.internal.editText('ref', ref);
            obj.push(sequence('', 'first', ref, '', '', '', ''));
        end

        function obj = addThen(obj, ref, varargin)
            if nargin < 2, ref = []; end
            defaults = struct('action', [], 'type', [], 'kind', 'action', ...
                'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, ...
                {'action', 'type', 'kind', 'multiplicity'}, varargin{:});
            ref = opensysml.internal.editText('ref', ref, true);
            values = {'action', options.action; 'type', options.type; ...
                'kind', options.kind; 'multiplicity', options.multiplicity};
            for i = 1:size(values, 1)
                options.(values{i, 1}) = opensysml.internal.editText( ...
                    values{i, 1}, values{i, 2}, true);
            end
            if isempty(ref) == isempty(options.action)
                opensysml.internal.raise('opensysml:argument', ...
                    'exactly one of ref and action is required');
            end
            if ~isempty(ref) && ...
                    (~isempty(options.type) || ...
                    (~isempty(options.kind) && ~strcmp(options.kind, 'action')))
                opensysml.internal.raise('opensysml:argument', ...
                    'a then reference takes no type or kind');
            end
            if ~isempty(ref)
                extra = struct();
                if ~isempty(options.multiplicity)
                    extra.multiplicity = options.multiplicity;
                end
                item = sequence('', 'then', ref, '', '', '', '', extra);
            else
                kind = options.kind;
                if isempty(kind), kind = 'action'; end
                extra = struct();
                if ~isempty(options.multiplicity)
                    extra.multiplicity = options.multiplicity;
                end
                item = sequence('', 'then', '', kind, options.action, ...
                    textOrEmpty(options.type), '', extra);
            end
            obj.push(item);
        end

        function obj = addAction(obj, name, varargin)
            if nargin < 2, name = []; end
            defaults = struct('type', [], 'kind', 'action');
            options = opensysml.internal.editOptions(defaults, {'type', 'kind'}, varargin{:});
            name = opensysml.internal.editText('name', name, true);
            options.type = opensysml.internal.editText('type', options.type, true);
            options.kind = opensysml.internal.editText('kind', options.kind, true);
            kind = textOrDefault(options.kind, 'action');
            obj.push(sequence('', '', '', kind, textOrEmpty(name), ...
                textOrEmpty(options.type), ''));
        end

        function obj = addAccept(obj, payload, varargin)
            defaults = struct('type', [], 'via', [], 'then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {'type', 'via'}, varargin{:});
            payload = opensysml.internal.editText('payload', payload);
            options.type = opensysml.internal.editText('type', options.type, true);
            options.via = opensysml.internal.editText('via', options.via, true);
            obj.addStatement('accept', options.then, options.multiplicity, ...
                struct('type', textOrEmpty(options.type), 'parameter', payload, ...
                'via', textOrEmpty(options.via)));
        end

        function obj = addSend(obj, payload, varargin)
            defaults = struct('to', [], 'via', [], 'then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {'to', 'via'}, varargin{:});
            payload = opensysml.internal.editText('payload', payload);
            options.to = opensysml.internal.editText('to', options.to, true);
            options.via = opensysml.internal.editText('via', options.via, true);
            obj.addStatement('send', options.then, options.multiplicity, ...
                struct('value', payload, 'target', textOrEmpty(options.to), ...
                'via', textOrEmpty(options.via)));
        end

        function obj = addAssign(obj, target, value, varargin)
            defaults = struct('then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {}, varargin{:});
            target = opensysml.internal.editText('target', target);
            value = opensysml.internal.editText('value', value);
            obj.addStatement('assign', options.then, options.multiplicity, ...
                struct('target', target, 'value', value));
        end

        function obj = addIf(obj, condition, body, varargin)
            defaults = struct('elseBody', [], 'then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {'elseBody'}, varargin{:});
            condition = opensysml.internal.editText('condition', condition);
            bodyItems = bodyItemsOf(body, 'body');
            elseItems = {};
            if ~isempty(options.elseBody)
                elseItems = bodyItemsOf(options.elseBody, 'elseBody');
            end
            obj.addStatement('if', options.then, options.multiplicity, ...
                struct('condition', condition, 'body', {bodyItems}, ...
                'elseBody', {elseItems}));
        end

        function obj = addWhile(obj, condition, body, varargin)
            defaults = struct('until', [], 'then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {'until'}, varargin{:});
            condition = opensysml.internal.editText('condition', condition);
            options.until = opensysml.internal.editText('until', options.until, true);
            bodyItems = bodyItemsOf(body, 'body');
            obj.addStatement('while', options.then, options.multiplicity, ...
                struct('condition', condition, 'until', textOrEmpty(options.until), ...
                'body', {bodyItems}));
        end

        function obj = addLoop(obj, body, varargin)
            defaults = struct('until', [], 'then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {'until'}, varargin{:});
            options.until = opensysml.internal.editText('until', options.until, true);
            bodyItems = bodyItemsOf(body, 'body');
            obj.addStatement('loop', options.then, options.multiplicity, ...
                struct('until', textOrEmpty(options.until), 'body', {bodyItems}));
        end

        function obj = addFor(obj, variable, collection, body, varargin)
            defaults = struct('type', [], 'then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {'type'}, varargin{:});
            variable = opensysml.internal.editText('variable', variable);
            collection = opensysml.internal.editText('collection', collection);
            options.type = opensysml.internal.editText('type', options.type, true);
            bodyItems = bodyItemsOf(body, 'body');
            obj.addStatement('for', options.then, options.multiplicity, ...
                struct('parameter', variable, 'type', textOrEmpty(options.type), ...
                'value', collection, 'body', {bodyItems}));
        end

        function obj = addTerminate(obj, varargin)
            defaults = struct('occurrence', [], 'then', [], 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, {'occurrence'}, varargin{:});
            options.occurrence = opensysml.internal.editText( ...
                'occurrence', options.occurrence, true);
            obj.addStatement('terminate', options.then, options.multiplicity, ...
                struct('value', textOrEmpty(options.occurrence)));
        end

        function obj = addGuardedThen(obj, guard, ref)
            guard = opensysml.internal.editText('guard', guard);
            ref = opensysml.internal.editText('ref', ref);
            obj.push(sequence('', 'if', ref, '', '', '', '', ...
                struct('condition', guard)));
        end

        function obj = addElse(obj, ref)
            ref = opensysml.internal.editText('ref', ref);
            obj.push(sequence('', 'else', ref, '', '', '', ''));
        end
    end

    methods (Access = private)
        function addStatement(obj, kind, thenOption, multiplicity, extra)
            if isempty(thenOption)
                if isempty(obj.itemList), keyword = ''; else, keyword = 'then'; end
            elseif islogical(thenOption) && isscalar(thenOption)
                if thenOption, keyword = 'then'; else, keyword = ''; end
            else
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('then must be bool or None, not %s', valueType(thenOption)));
            end
            multiplicity = opensysml.internal.editText( ...
                'multiplicity', multiplicity, true);
            if ~isempty(multiplicity) && ~strcmp(keyword, 'then')
                opensysml.internal.raise('opensysml:argument', ...
                    'multiplicity requires then=True');
            end
            if ~isempty(multiplicity), extra.multiplicity = multiplicity; end
            item = sequence('', keyword, '', kind, '', '', '', extra);
            obj.push(item);
        end

        function push(obj, item)
            obj.itemList{end+1} = item;
            opensysml.internal.validateSequenceDepth(obj.itemList);
        end
    end
end

function item = sequence(owner, keyword, ref, kind, name, typeName, after, extra)
    if nargin < 8, extra = struct(); end
    item = opensysml.internal.sequenceEdit(owner, keyword, ref, kind, ...
        name, typeName, after, extra);
end

function items = bodyItemsOf(body, label)
    if ~isa(body, 'opensysml.Body')
        opensysml.internal.raise('opensysml:argument', sprintf( ...
            '%s must be Body, not %s', label, valueType(body)));
    end
    items = body.operations();
end

function value = textOrEmpty(value)
    if isempty(value), value = ''; end
end

function value = textOrDefault(value, default)
    if isempty(value), value = default; end
end

function name = valueType(value)
    if isempty(value), name = 'NoneType';
    elseif islogical(value), name = 'bool';
    elseif isnumeric(value) && isscalar(value) && isfinite(double(value)) && ...
            double(value) == fix(double(value)), name = 'int';
    elseif isnumeric(value), name = 'float';
    elseif iscell(value), name = 'list';
    elseif isstruct(value) || isa(value, 'containers.Map'), name = 'dict';
    else, name = class(value);
    end
end
