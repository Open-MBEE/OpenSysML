classdef Editor < handle
%EDITOR Collect source-preserving edits for one parsed model.

    properties (SetAccess = private)
        applied = false
    end

    properties (Access = private)
        model
        operationList = {}
    end

    methods
        function obj = Editor(model)
            if ~isa(model, 'opensysml.Model')
                opensysml.internal.raise('opensysml:argument', ...
                    'Editor needs an opensysml.Model');
            end
            obj.model = model;
        end

        function count = numOperations(obj)
            count = numel(obj.operationList);
        end

        function obj = setValue(obj, target, value)
            if isstring(value) && isscalar(value), value = char(value); end
            if ~ischar(value)
                opensysml.internal.raise('opensysml:argument', sprintf( ...
                    'value must be SysML notation for an expression, not %s: write it as it should read in the file', ...
                    valueType(value)));
            end
            fields = struct('target', opensysml.internal.editTargetId(target), ...
                'value', value);
            obj.push('setValue', fields);
        end

        function obj = rename(obj, target, newName)
            newName = opensysml.internal.editText('newName', newName, false, 'a name');
            obj.push('rename', struct('target', opensysml.internal.editTargetId(target), ...
                'newName', newName));
        end

        function obj = addMember(obj, owner, kind, name, varargin)
            defaults = memberOptions();
            positional = {'type', 'multiplicity', 'value', 'specializes', ...
                'abstract', 'redefines', 'default', 'direction', 'metadata', ...
                'expression', 'doc'};
            options = opensysml.internal.editOptions(defaults, positional, varargin{:});
            kind = opensysml.internal.editText('kind', kind);
            name = opensysml.internal.editText('name', name);
            options.type = opensysml.internal.editText('type', options.type, true);
            options.multiplicity = opensysml.internal.editText( ...
                'multiplicity', options.multiplicity, true);
            options.value = opensysml.internal.editText('value', options.value, true);
            options.expression = opensysml.internal.editText( ...
                'expression', options.expression, true);
            options.direction = opensysml.internal.editText( ...
                'direction', options.direction, true);
            options.doc = opensysml.internal.editText('doc', options.doc, true, 'text');
            options.specializes = notationReferences('specializes', options.specializes);
            options.redefines = notationReferences('redefines', options.redefines);
            options.metadata = notationReferences('metadata', options.metadata);
            requireBoolean(options.abstract, 'abstract');
            requireBoolean(options.default, 'default');
            fields = struct('owner', ownerId(owner), 'kind', kind, 'name', name, ...
                'type', textOrEmpty(options.type), ...
                'multiplicity', textOrEmpty(options.multiplicity), ...
                'value', textOrEmpty(options.value), ...
                'specializes', {options.specializes}, ...
                'isAbstract', options.abstract, ...
                'redefines', {options.redefines}, ...
                'isDefault', options.default, ...
                'direction', textOrEmpty(options.direction), ...
                'metadataPrefixes', {options.metadata}, ...
                'bodyExpression', textOrEmpty(options.expression), ...
                'doc', textOrEmpty(options.doc));
            obj.push('addMember', fields);
        end

        function obj = addObjective(obj, owner, varargin)
            defaults = struct('name', [], 'type', []);
            options = opensysml.internal.editOptions(defaults, {'name', 'type'}, varargin{:});
            options.name = opensysml.internal.editText('name', options.name, true);
            options.type = opensysml.internal.editText('type', options.type, true);
            obj.addMember(owner, 'objective', textOrEmpty(options.name), ...
                'type', options.type);
        end

        function obj = addVerify(obj, owner, requirement)
            requirement = opensysml.internal.editText('requirement', requirement);
            obj.push('addVerify', struct('owner', ownerId(owner), ...
                'requirement', requirement));
        end

        function obj = addMetadata(obj, owner, metadataType, varargin)
            defaults = struct('values', [], 'name', [], 'about', [], 'shorthand', false);
            options = opensysml.internal.editOptions(defaults, ...
                {'values', 'name', 'about', 'shorthand'}, varargin{:});
            metadataType = opensysml.internal.editText('metadataType', metadataType);
            options.name = opensysml.internal.editText('name', options.name, true);
            requireBoolean(options.shorthand, 'shorthand');
            about = notationReferences('about', options.about);
            values = metadataValues(options.values);
            fields = struct('owner', ownerId(owner), 'metadataType', metadataType, ...
                'name', textOrEmpty(options.name), 'about', {about}, ...
                'values', {values}, 'shorthand', options.shorthand);
            obj.push('addMetadata', fields);
        end

        function obj = addMetadataPrefix(obj, target, metadataType)
            metadataType = opensysml.internal.editText('metadataType', metadataType);
            obj.push('addMetadataPrefix', struct( ...
                'target', opensysml.internal.editTargetId(target), ...
                'metadataType', metadataType));
        end

        function obj = addDocumentation(obj, target, body, varargin)
            defaults = struct('name', [], 'locale', [], 'replace', false);
            options = opensysml.internal.editOptions(defaults, ...
                {'name', 'locale', 'replace'}, varargin{:});
            body = opensysml.internal.editText('body', body, false, 'text');
            options.name = opensysml.internal.editText('name', options.name, true, 'text');
            options.locale = opensysml.internal.editText('locale', options.locale, true, 'text');
            requireBoolean(options.replace, 'replace', 'a bool');
            obj.push('addDocumentation', struct( ...
                'target', opensysml.internal.editTargetId(target), ...
                'body', body, 'name', textOrEmpty(options.name), ...
                'locale', textOrEmpty(options.locale), 'replace', options.replace));
        end

        function obj = addComment(obj, owner, body, varargin)
            defaults = struct('name', [], 'about', [], 'locale', []);
            options = opensysml.internal.editOptions(defaults, ...
                {'name', 'about', 'locale'}, varargin{:});
            body = opensysml.internal.editText('body', body, false, 'text');
            options.name = opensysml.internal.editText('name', options.name, true, 'text');
            options.locale = opensysml.internal.editText('locale', options.locale, true, 'text');
            if ischar(options.about) || (isstring(options.about) && isscalar(options.about))
                opensysml.internal.raise('opensysml:argument', ...
                    'about must be a sequence of names or symbols, not one name');
            end
            about = notationReferences('about', options.about);
            obj.push('addComment', struct('owner', ownerId(owner), 'body', body, ...
                'name', textOrEmpty(options.name), 'about', {about}, ...
                'locale', textOrEmpty(options.locale)));
        end

        function obj = addNote(obj, target, text)
            text = opensysml.internal.editText('text', text, false, 'text');
            if ~isempty(strfind(text, sprintf('\n'))) || ...
                    ~isempty(strfind(text, sprintf('\r')))
                opensysml.internal.raise('opensysml:argument', ...
                    'a note is one line: its text may not contain a line break');
            end
            obj.push('addNote', struct( ...
                'target', opensysml.internal.editTargetId(target), 'text', text));
        end

        function obj = addSatisfy(obj, owner, requirement, varargin)
            defaults = struct('by', [], 'asserted', false, 'negated', false);
            options = opensysml.internal.editOptions(defaults, ...
                {'by', 'asserted', 'negated'}, varargin{:});
            requirement = opensysml.internal.editText('requirement', requirement);
            options.by = opensysml.internal.editText('by', options.by, true);
            if ~isBoolean(options.asserted) || ~isBoolean(options.negated)
                opensysml.internal.raise('opensysml:argument', ...
                    'asserted and negated must be bool');
            end
            obj.push('addSatisfy', struct('owner', ownerId(owner), ...
                'requirement', requirement, ...
                'satisfyingFeature', textOrEmpty(options.by), ...
                'isAsserted', options.asserted, 'isNegated', options.negated));
        end

        function obj = addRequirementConstraint(obj, owner, kind, expression, varargin)
            defaults = struct('name', []);
            options = opensysml.internal.editOptions(defaults, {'name'}, varargin{:});
            kind = opensysml.internal.editText('kind', kind);
            expression = opensysml.internal.editText('expression', expression);
            options.name = opensysml.internal.editText('name', options.name, true);
            obj.push('addRequirementConstraint', struct('owner', ownerId(owner), ...
                'kind', kind, 'expression', expression, ...
                'name', textOrEmpty(options.name)));
        end

        function obj = addRequireConstraint(obj, owner, expression, varargin)
            obj.addRequirementConstraint(owner, 'require', expression, varargin{:});
        end

        function obj = addAssumeConstraint(obj, owner, expression, varargin)
            obj.addRequirementConstraint(owner, 'assume', expression, varargin{:});
        end

        function obj = addTransition(obj, owner, source, target, varargin)
            defaults = struct('name', [], 'trigger', [], 'guard', [], 'effect', []);
            options = opensysml.internal.editOptions(defaults, ...
                {'name', 'trigger', 'guard', 'effect'}, varargin{:});
            source = opensysml.internal.editText('source', source);
            target = opensysml.internal.editText('target', target);
            for key = {'name', 'trigger', 'guard', 'effect'}
                name = key{1};
                options.(name) = opensysml.internal.editText(name, options.(name), true);
            end
            obj.push('addTransition', struct('owner', ownerId(owner), ...
                'name', textOrEmpty(options.name), 'source', source, 'target', target, ...
                'trigger', textOrEmpty(options.trigger), 'guard', textOrEmpty(options.guard), ...
                'effect', textOrEmpty(options.effect), 'initial', false));
        end

        function obj = addEntryTransition(obj, owner, target)
            target = opensysml.internal.editText('target', target);
            obj.push('addTransition', struct('owner', ownerId(owner), 'name', '', ...
                'source', '', 'target', target, 'trigger', '', 'guard', '', ...
                'effect', '', 'initial', true));
        end

        function obj = addFirst(obj, owner, ref, varargin)
            defaults = struct('after', []);
            options = opensysml.internal.editOptions(defaults, {'after'}, varargin{:});
            ref = opensysml.internal.editText('ref', ref);
            options.after = opensysml.internal.editText('after', options.after, true, 'a member name');
            obj.pushSequence(owner, 'first', ref, '', '', '', ...
                textOrEmpty(options.after), struct());
        end

        function obj = addThen(obj, owner, varargin)
            defaults = struct('ref', [], 'action', [], 'type', [], 'after', [], ...
                'kind', 'action', 'multiplicity', []);
            options = opensysml.internal.editOptions(defaults, ...
                {'ref', 'action', 'type', 'after', 'kind', 'multiplicity'}, varargin{:});
            for key = {'ref', 'action', 'type', 'after', 'kind', 'multiplicity'}
                name = key{1};
                options.(name) = opensysml.internal.editText(name, options.(name), true);
            end
            if isempty(options.ref) == isempty(options.action)
                opensysml.internal.raise('opensysml:argument', ...
                    'exactly one of ref and action is required');
            end
            if ~isempty(options.ref) && (~isempty(options.type) || ...
                    ~isempty(options.kind) && ~strcmp(options.kind, 'action'))
                opensysml.internal.raise('opensysml:argument', ...
                    'a then reference takes no type or kind');
            end
            after = textOrEmpty(options.after);
            if ~isempty(options.ref)
                extra = struct();
                if ~isempty(options.multiplicity)
                    extra.multiplicity = options.multiplicity;
                end
                obj.pushSequence(owner, 'then', options.ref, '', '', '', after, extra);
            else
                kind = textOrDefault(options.kind, 'action');
                extra = struct();
                if ~isempty(options.multiplicity)
                    extra.multiplicity = options.multiplicity;
                end
                obj.pushSequence(owner, 'then', '', kind, options.action, ...
                    textOrEmpty(options.type), after, extra);
            end
        end

        function obj = addAccept(obj, owner, payload, varargin)
            defaults = struct('type', [], 'via', [], 'then', true, ...
                'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, ...
                {'type', 'via'}, varargin{:});
            payload = opensysml.internal.editText('payload', payload);
            options.type = opensysml.internal.editText('type', options.type, true);
            options.via = opensysml.internal.editText('via', options.via, true);
            obj.addActionStatement(owner, 'accept', options.then, ...
                options.multiplicity, options.after, '', '', ...
                textOrEmpty(options.type), struct('parameter', payload, ...
                'via', textOrEmpty(options.via)));
        end

        function obj = addSend(obj, owner, payload, varargin)
            defaults = struct('to', [], 'via', [], 'then', true, ...
                'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, ...
                {'to', 'via'}, varargin{:});
            payload = opensysml.internal.editText('payload', payload);
            options.to = opensysml.internal.editText('to', options.to, true);
            options.via = opensysml.internal.editText('via', options.via, true);
            obj.addActionStatement(owner, 'send', options.then, ...
                options.multiplicity, options.after, '', '', '', ...
                struct('value', payload, 'target', textOrEmpty(options.to), ...
                'via', textOrEmpty(options.via)));
        end

        function obj = addAssign(obj, owner, target, value, varargin)
            defaults = struct('then', true, 'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, {}, varargin{:});
            target = opensysml.internal.editText('target', target);
            value = opensysml.internal.editText('value', value);
            obj.addActionStatement(owner, 'assign', options.then, ...
                options.multiplicity, options.after, '', '', '', ...
                struct('target', target, 'value', value));
        end

        function obj = addIf(obj, owner, condition, body, varargin)
            defaults = struct('elseBody', [], 'then', true, ...
                'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, {'elseBody'}, varargin{:});
            condition = opensysml.internal.editText('condition', condition);
            bodyItems = bodyItemsOf(body, 'body');
            elseItems = {};
            if ~isempty(options.elseBody)
                elseItems = bodyItemsOf(options.elseBody, 'elseBody');
            end
            obj.addActionStatement(owner, 'if', options.then, ...
                options.multiplicity, options.after, '', '', '', ...
                struct('condition', condition, 'body', {bodyItems}, ...
                'elseBody', {elseItems}));
        end

        function obj = addWhile(obj, owner, condition, body, varargin)
            defaults = struct('until', [], 'then', true, ...
                'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, {'until'}, varargin{:});
            condition = opensysml.internal.editText('condition', condition);
            options.until = opensysml.internal.editText('until', options.until, true);
            bodyItems = bodyItemsOf(body, 'body');
            obj.addActionStatement(owner, 'while', options.then, ...
                options.multiplicity, options.after, '', '', '', ...
                struct('condition', condition, 'until', textOrEmpty(options.until), ...
                'body', {bodyItems}));
        end

        function obj = addLoop(obj, owner, body, varargin)
            defaults = struct('until', [], 'then', true, ...
                'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, {'until'}, varargin{:});
            options.until = opensysml.internal.editText('until', options.until, true);
            bodyItems = bodyItemsOf(body, 'body');
            obj.addActionStatement(owner, 'loop', options.then, ...
                options.multiplicity, options.after, '', '', '', ...
                struct('until', textOrEmpty(options.until), 'body', {bodyItems}));
        end

        function obj = addFor(obj, owner, variable, collection, body, varargin)
            defaults = struct('type', [], 'then', true, ...
                'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, {'type'}, varargin{:});
            variable = opensysml.internal.editText('variable', variable);
            collection = opensysml.internal.editText('collection', collection);
            options.type = opensysml.internal.editText('type', options.type, true);
            bodyItems = bodyItemsOf(body, 'body');
            obj.addActionStatement(owner, 'for', options.then, ...
                options.multiplicity, options.after, '', '', ...
                textOrEmpty(options.type), struct('parameter', variable, ...
                'value', collection, 'body', {bodyItems}));
        end

        function obj = addTerminate(obj, owner, varargin)
            defaults = struct('occurrence', [], 'then', true, ...
                'multiplicity', [], 'after', []);
            options = opensysml.internal.editOptions(defaults, {'occurrence'}, varargin{:});
            options.occurrence = opensysml.internal.editText( ...
                'occurrence', options.occurrence, true);
            obj.addActionStatement(owner, 'terminate', options.then, ...
                options.multiplicity, options.after, '', '', '', ...
                struct('value', textOrEmpty(options.occurrence)));
        end

        function obj = addGuardedThen(obj, owner, guard, ref, varargin)
            defaults = struct('after', []);
            options = opensysml.internal.editOptions(defaults, {'after'}, varargin{:});
            guard = opensysml.internal.editText('guard', guard);
            ref = opensysml.internal.editText('ref', ref);
            options.after = opensysml.internal.editText('after', options.after, true);
            obj.pushSequence(owner, 'if', ref, '', '', '', ...
                textOrEmpty(options.after), struct('condition', guard));
        end

        function obj = addElse(obj, owner, ref, varargin)
            defaults = struct('after', []);
            options = opensysml.internal.editOptions(defaults, {'after'}, varargin{:});
            ref = opensysml.internal.editText('ref', ref);
            options.after = opensysml.internal.editText('after', options.after, true);
            obj.pushSequence(owner, 'else', ref, '', '', '', ...
                textOrEmpty(options.after), struct());
        end

        function obj = addImport(obj, owner, target, varargin)
            defaults = struct('visibility', [], 'recursive', false, ...
                'all', false, 'filter', []);
            options = opensysml.internal.editOptions(defaults, ...
                {'visibility', 'recursive', 'all', 'filter'}, varargin{:});
            target = opensysml.internal.editText('target', target);
            options.visibility = opensysml.internal.editText( ...
                'visibility', options.visibility, true);
            requireBoolean(options.recursive, 'recursive');
            requireBoolean(options.all, 'all');
            filters = textList('filter', options.filter);
            obj.push('addImport', struct('owner', ownerId(owner), ...
                'visibility', textOrEmpty(options.visibility), 'target', target, ...
                'isRecursive', options.recursive, 'isImportAll', options.all, ...
                'filters', {filters}));
        end

        function obj = addConnection(obj, owner, kind, fromEnd, toEnd, varargin)
            defaults = struct('name', [], 'type', []);
            options = opensysml.internal.editOptions(defaults, {'name', 'type'}, varargin{:});
            kind = opensysml.internal.editText('kind', kind);
            fromEnd = opensysml.internal.editText('from', fromEnd);
            toEnd = opensysml.internal.editText('to', toEnd);
            options.name = opensysml.internal.editText('name', options.name, true);
            options.type = opensysml.internal.editText('type', options.type, true);
            obj.push('addConnection', struct('owner', ownerId(owner), 'kind', kind, ...
                'fromEnd', fromEnd, 'toEnd', toEnd, ...
                'name', textOrEmpty(options.name), 'type', textOrEmpty(options.type)));
        end

        function obj = addAllocation(obj, owner, fromEnd, toEnd, varargin)
            obj.addConnection(owner, 'allocation', fromEnd, toEnd, varargin{:});
        end

        function obj = addFlow(obj, owner, fromEnd, toEnd, varargin)
            obj.addConnection(owner, 'flow', fromEnd, toEnd, varargin{:});
        end

        function obj = addSuccession(obj, owner, fromEnd, toEnd, varargin)
            obj.addConnection(owner, 'succession', fromEnd, toEnd, varargin{:});
        end

        function obj = deleteElement(obj, target, varargin)
            defaults = struct('cascade', false);
            options = opensysml.internal.editOptions(defaults, {'cascade'}, varargin{:});
            requireBoolean(options.cascade, 'cascade');
            obj.push('delete', struct( ...
                'target', opensysml.internal.editTargetId(target), ...
                'cascade', options.cascade));
        end

        function obj = move(obj, target, owner)
            obj.push('move', struct( ...
                'target', opensysml.internal.editTargetId(target), ...
                'owner', ownerId(owner)));
        end

        function obj = addPackage(obj, owner, name, varargin)
            obj.addMember(owner, 'package', name, varargin{:});
        end

        function obj = addPartDef(obj, owner, name, varargin)
            obj.addMember(owner, 'part def', name, varargin{:});
        end

        function obj = addPart(obj, owner, name, varargin)
            obj.addMember(owner, 'part', name, varargin{:});
        end

        function obj = addAttributeDef(obj, owner, name, varargin)
            obj.addMember(owner, 'attribute def', name, varargin{:});
        end

        function obj = addAttribute(obj, owner, name, varargin)
            obj.addMember(owner, 'attribute', name, varargin{:});
        end

        function obj = addItemDef(obj, owner, name, varargin)
            obj.addMember(owner, 'item def', name, varargin{:});
        end

        function obj = addItem(obj, owner, name, varargin)
            obj.addMember(owner, 'item', name, varargin{:});
        end

        function obj = addPortDef(obj, owner, name, varargin)
            obj.addMember(owner, 'port def', name, varargin{:});
        end

        function obj = addPort(obj, owner, name, varargin)
            obj.addMember(owner, 'port', name, varargin{:});
        end

        function obj = addClass(obj, owner, name, varargin)
            obj.addMember(owner, 'class', name, varargin{:});
        end

        function obj = addStruct(obj, owner, name, varargin)
            obj.addMember(owner, 'struct', name, varargin{:});
        end

        function obj = addDatatype(obj, owner, name, varargin)
            obj.addMember(owner, 'datatype', name, varargin{:});
        end

        function obj = addClassifier(obj, owner, name, varargin)
            obj.addMember(owner, 'classifier', name, varargin{:});
        end

        function obj = addFeature(obj, owner, name, varargin)
            obj.addMember(owner, 'feature', name, varargin{:});
        end

        function obj = addAssoc(obj, owner, name, varargin)
            obj.addMember(owner, 'assoc', name, varargin{:});
        end

        function obj = addBehavior(obj, owner, name, varargin)
            obj.addMember(owner, 'behavior', name, varargin{:});
        end

        function obj = addFunction(obj, owner, name, varargin)
            obj.addMember(owner, 'function', name, varargin{:});
        end

        function obj = addPredicate(obj, owner, name, varargin)
            obj.addMember(owner, 'predicate', name, varargin{:});
        end

        function obj = addInteraction(obj, owner, name, varargin)
            obj.addMember(owner, 'interaction', name, varargin{:});
        end

        function obj = addMetaclass(obj, owner, name, varargin)
            obj.addMember(owner, 'metaclass', name, varargin{:});
        end

        function obj = addCalcDef(obj, owner, name, varargin)
            options = calcOptions(varargin{:});
            addMemberOptions(obj, owner, 'calc def', name, options);
            addCalculationParameters(obj, owner, name, options);
        end

        function obj = addCalc(obj, owner, name, varargin)
            options = calcOptions(varargin{:});
            addMemberOptions(obj, owner, 'calc', name, options);
            addCalculationParameters(obj, owner, name, options);
        end

        function obj = addParameter(obj, owner, direction, name, varargin)
            defaults = memberOptions();
            defaults.kind = [];
            positional = {'type', 'kind'};
            options = opensysml.internal.editOptions(defaults, positional, varargin{:});
            kind = textOrEmpty(opensysml.internal.editText( ...
                'kind', options.kind, true));
            options.direction = direction;
            addMemberOptions(obj, owner, kind, name, options);
        end

        function obj = addReturn(obj, owner, varargin)
            defaults = memberOptions();
            defaults.name = '';
            options = opensysml.internal.editOptions(defaults, {'name'}, varargin{:});
            addMemberOptions(obj, owner, 'return', options.name, options);
        end

        function obj = addActionDef(obj, owner, name, varargin)
            options = actionOptions(varargin{:});
            addMemberOptions(obj, owner, 'action def', name, options);
            addActionParameters(obj, owner, name, options.inputs, options.outputs);
        end

        function obj = addAction(obj, owner, name, varargin)
            options = actionOptions(varargin{:});
            addMemberOptions(obj, owner, 'action', name, options);
            addActionParameters(obj, owner, name, options.inputs, options.outputs);
        end

        function obj = addPerformAction(obj, owner, name, varargin)
            defaults = memberOptions();
            options = opensysml.internal.editOptions(defaults, {'type'}, varargin{:});
            addMemberOptions(obj, owner, 'perform action', name, options);
        end

        function obj = addPerform(obj, owner, action, varargin)
            defaults = struct('doc', []);
            options = opensysml.internal.editOptions(defaults, {'doc'}, varargin{:});
            obj.addMember(owner, 'perform', action, 'doc', options.doc);
        end

        function obj = addExhibitState(obj, owner, name, varargin)
            defaults = struct('type', []);
            options = opensysml.internal.editOptions(defaults, {'type'}, varargin{:});
            obj.addMember(owner, 'exhibit state', name, 'type', options.type);
        end

        function obj = addExhibit(obj, owner, state)
            obj.addMember(owner, 'exhibit', state);
        end

        function obj = addStateAction(obj, owner, kind, name, varargin)
            kind = opensysml.internal.editText('kind', kind);
            if ~any(strcmp(kind, {'entry', 'do', 'exit'}))
                opensysml.internal.raise('opensysml:argument', ...
                    'kind must be ''entry'', ''do'' or ''exit''');
            end
            defaults = struct('type', []);
            options = opensysml.internal.editOptions(defaults, {'type'}, varargin{:});
            obj.addMember(owner, [kind ' action'], name, 'type', options.type);
        end

        function obj = addStateDef(obj, owner, name, varargin)
            obj.addMember(owner, 'state def', name, varargin{:});
        end

        function obj = addState(obj, owner, name, varargin)
            obj.addMember(owner, 'state', name, varargin{:});
        end

        function obj = addConstraintDef(obj, owner, name, varargin)
            defaults = memberOptions();
            options = opensysml.internal.editOptions(defaults, {'expression'}, varargin{:});
            addMemberOptions(obj, owner, 'constraint def', name, options);
        end

        function obj = addConstraint(obj, owner, name, varargin)
            defaults = memberOptions();
            options = opensysml.internal.editOptions(defaults, {'expression'}, varargin{:});
            addMemberOptions(obj, owner, 'constraint', name, options);
        end

        function obj = addAssertConstraint(obj, owner, varargin)
            defaults = memberOptions();
            defaults.name = [];
            defaults.negated = false;
            options = opensysml.internal.editOptions(defaults, ...
                {'name', 'type', 'expression', 'negated'}, varargin{:});
            requireBoolean(options.negated, 'negated');
            kind = 'assert constraint';
            if options.negated, kind = 'assert not constraint'; end
            addMemberOptions(obj, owner, kind, textOrEmpty(options.name), options);
        end

        function obj = addAssert(obj, owner, ref, varargin)
            defaults = struct('negated', false);
            options = opensysml.internal.editOptions(defaults, {'negated'}, varargin{:});
            ref = opensysml.internal.editText('ref', ref);
            requireBoolean(options.negated, 'negated');
            kind = 'assert';
            if options.negated, kind = 'assert not'; end
            obj.addMember(owner, kind, ref);
        end

        function obj = addRequirementDef(obj, owner, name, varargin)
            obj.addMember(owner, 'requirement def', name, varargin{:});
        end

        function obj = addRequirement(obj, owner, name, varargin)
            obj.addMember(owner, 'requirement', name, varargin{:});
        end

        function result = apply(obj)
            if obj.applied
                opensysml.internal.raise('opensysml:argument', ...
                    ['this editor has already been applied: it describes an edit of the model it was made from, ' ...
                    'so build another editor from the edited model rather than applying this one twice']);
            end
            if isempty(obj.operationList)
                opensysml.internal.raise('opensysml:diagnostics:edit:noOperations', ...
                    'this editor has no operations: add an edit before applying it', ...
                    {}, struct('failure', 'EDIT_FAILURE_NO_OPERATIONS'));
            end
            result = opensysml.applyEdits(obj.model, obj.operations());
            obj.applied = true;
        end
    end

    methods (Hidden)
        function operations = operations(obj)
            operations = obj.operationList;
            for i = 1:numel(operations)
                arm = fieldnames(operations{i});
                if numel(arm) == 1 && strcmp(arm{1}, 'addSequence')
                    opensysml.internal.validateSequenceDepth({operations{i}.addSequence});
                end
            end
        end
    end

    methods (Access = private)
        function pushSequence(obj, owner, keyword, ref, memberKind, memberName, typeName, after, extra)
            item = opensysml.internal.sequenceEdit(ownerId(owner), keyword, ref, ...
                memberKind, memberName, typeName, after, extra);
            obj.push('addSequence', item);
        end

        function addActionStatement(obj, owner, kind, then, multiplicity, after, ...
                ref, memberName, typeName, extra)
            if ~islogical(then) || ~isscalar(then)
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('then must be bool, not %s', valueType(then)));
            end
            multiplicity = opensysml.internal.editText('multiplicity', multiplicity, true);
            after = opensysml.internal.editText('after', after, true);
            if ~isempty(multiplicity) && ~then
                opensysml.internal.raise('opensysml:argument', ...
                    'multiplicity requires then=True');
            end
            keyword = '';
            if then, keyword = 'then'; end
            if ~isempty(multiplicity), extra.multiplicity = multiplicity; end
            obj.pushSequence(owner, keyword, ref, kind, memberName, typeName, ...
                textOrEmpty(after), extra);
        end

        function push(obj, arm, fields)
            obj.ensureOpen();
            obj.operationList{end+1} = opensysml.internal.editOperation(arm, fields);
        end

        function ensureOpen(obj)
            if obj.applied
                opensysml.internal.raise('opensysml:argument', ...
                    'this editor has already been applied: build another editor from the edited model to edit further');
            end
        end
    end
end

function options = memberOptions()
    options = struct('type', [], 'multiplicity', [], 'value', [], ...
        'specializes', [], 'abstract', false, 'redefines', [], 'default', false, ...
        'direction', [], 'metadata', [], 'expression', [], 'doc', []);
end

function id = ownerId(owner)
    if ischar(owner) || (isstring(owner) && isscalar(owner))
        id = char(owner);
    else
        id = opensysml.internal.editTargetId(owner);
    end
end

function values = notationReferences(label, raw)
    values = {};
    if isempty(raw), return; end
    if ischar(raw) || (isstring(raw) && isscalar(raw))
        raw = {raw};
    elseif ~iscell(raw)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a notation string or sequence of strings', label));
    end
    for i = 1:numel(raw)
        value = raw{i};
        if isstring(value) && isscalar(value), value = char(value); end
        if ~ischar(value)
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s must contain only notation strings', label));
        end
        values{end+1} = value;
    end
end

function values = metadataValues(raw)
    values = {};
    if isempty(raw), return; end
    if isa(raw, 'containers.Map')
        names = keys(raw);
        pairs = cell(1, numel(names));
        for i = 1:numel(names)
            pairs{i} = {names{i}, raw(names{i})};
        end
    elseif isstruct(raw) && isscalar(raw)
        names = fieldnames(raw);
        pairs = cell(1, numel(names));
        for i = 1:numel(names)
            pairs{i} = {names{i}, raw.(names{i})};
        end
    elseif iscell(raw)
        pairs = raw;
    else
        opensysml.internal.raise('opensysml:argument', ...
            'values must be a mapping or a sequence of (feature, value) pairs');
    end
    for i = 1:numel(pairs)
        pair = pairs{i};
        if ~iscell(pair) || numel(pair) ~= 2
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('values[%d] must be a pair of strings', i-1));
        end
        feature = pair{1};
        value = pair{2};
        if isstring(feature) && isscalar(feature), feature = char(feature); end
        if isstring(value) && isscalar(value), value = char(value); end
        if ~ischar(feature) || ~ischar(value)
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('values[%d] feature and value must be strings', i-1));
        end
        values{end+1} = struct('feature', feature, 'value', value);
    end
end

function values = textList(label, raw)
    values = {};
    if isempty(raw), return; end
    if ischar(raw) || (isstring(raw) && isscalar(raw)), raw = {raw}; end
    if ~iscell(raw)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a sequence of notation text', label));
    end
    for i = 1:numel(raw)
        values{end+1} = opensysml.internal.editText(label, raw{i});
    end
end

function requireBoolean(value, label, wording)
    if nargin < 3, wording = 'bool'; end
    if ~isBoolean(value)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be %s', label, wording));
    end
end

function tf = isBoolean(value)
    tf = islogical(value) && isscalar(value);
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

function options = calcOptions(varargin)
    defaults = memberOptions();
    defaults.inputs = [];
    defaults.returnType = [];
    defaults.returnExpression = [];
    options = opensysml.internal.editOptions(defaults, ...
        {'inputs', 'returnType', 'returnExpression', 'expression'}, varargin{:});
    options.inputs = parameterPairs(options.inputs, 'inputs');
    options.returnType = optionalNotation('returnType', options.returnType);
    options.returnExpression = optionalNotation( ...
        'returnExpression', options.returnExpression);
    options.expression = optionalNotation('expression', options.expression);
    if ~isempty(options.expression) && ~isempty(options.returnExpression)
        opensysml.internal.raise('opensysml:argument', ...
            'expression and returnExpression both bind the result; give one');
    end
    if ~isempty(options.returnExpression) && isempty(options.returnType)
        opensysml.internal.raise('opensysml:argument', ...
            'returnExpression requires returnType');
    end
end

function options = actionOptions(varargin)
    defaults = memberOptions();
    defaults.inputs = [];
    defaults.outputs = [];
    options = opensysml.internal.editOptions(defaults, ...
        {'inputs', 'outputs'}, varargin{:});
    options.inputs = parameterPairs(options.inputs, 'inputs');
    options.outputs = parameterPairs(options.outputs, 'outputs');
end

function pairs = parameterPairs(raw, label)
    pairs = {};
    if isempty(raw), return; end
    if ~iscell(raw)
        opensysml.internal.raise('opensysml:argument', sprintf( ...
            '%s must be a list of 2-tuples of strings, not %s', ...
            label, valueType(raw)));
    end
    pairs = cell(1, numel(raw));
    for i = 1:numel(raw)
        pair = raw{i};
        if ~iscell(pair) || numel(pair) ~= 2
            opensysml.internal.raise('opensysml:argument', sprintf( ...
                '%s[%d] must be a 2-tuple of strings', label, i-1));
        end
        if ~(ischar(pair{1}) || (isstring(pair{1}) && isscalar(pair{1}))) || ...
                ~(ischar(pair{2}) || (isstring(pair{2}) && isscalar(pair{2})))
            opensysml.internal.raise('opensysml:argument', sprintf( ...
                '%s[%d] name and type must be strings', label, i-1));
        end
        pairs{i} = {char(pair{1}), char(pair{2})};
    end
end

function addMemberOptions(obj, owner, kind, name, options)
    defaults = memberOptions();
    names = fieldnames(defaults);
    args = cell(1, 2 * numel(names));
    for i = 1:numel(names)
        args{2*i-1} = names{i};
        if isfield(options, names{i})
            args{2*i} = options.(names{i});
        else
            args{2*i} = defaults.(names{i});
        end
    end
    obj.addMember(owner, kind, name, args{:});
end

function addCalculationParameters(obj, owner, name, options)
    owner = ownerId(owner);
    name = opensysml.internal.editText('name', name);
    if isempty(owner), qualified = name;
    else, qualified = [owner '::' name];
    end
    for i = 1:numel(options.inputs)
        pair = options.inputs{i};
        obj.addParameter(qualified, 'in', pair{1}, 'type', pair{2});
    end
    if ~isempty(options.returnType) || ~isempty(options.returnExpression)
        obj.addReturn(qualified, 'type', textOrEmpty(options.returnType), ...
            'value', textOrEmpty(options.returnExpression));
    end
end

function addActionParameters(obj, owner, name, inputs, outputs)
    owner = ownerId(owner);
    name = opensysml.internal.editText('name', name);
    if isempty(owner), qualified = name;
    else, qualified = [owner '::' name];
    end
    for i = 1:numel(inputs)
        pair = inputs{i};
        obj.addParameter(qualified, 'in', pair{1}, 'type', pair{2});
    end
    for i = 1:numel(outputs)
        pair = outputs{i};
        obj.addParameter(qualified, 'out', pair{1}, 'type', pair{2});
    end
end

function value = optionalNotation(label, value)
    value = opensysml.internal.editText(label, value, true);
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
