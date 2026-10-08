function result = applyEdits(model, operations)
%APPLYEDITS Apply one cell of protobuf edit-operation structs to a model.

    if ~isa(model, 'opensysml.Model')
        opensysml.internal.raise('opensysml:argument', ...
            'applyEdits needs an opensysml.Model');
    end
    if ~iscell(operations)
        opensysml.internal.raise('opensysml:argument', ...
            'operations must be a cell of edit-operation structs');
    end
    if isempty(model.connection)
        opensysml.internal.raise('opensysml:argument', ...
            'the model has no connection');
    end
    connection = model.connection;
    requested = {'apply_edits'};
    connection.require('apply_edits');
    if numel(model.documents) > 1
        connection.require('edit_documents');
        requested{end+1} = 'edit_documents';
    end
    deferred = struct('actionBody', false, 'memberModifiers', false, ...
        'implicitParameters', false);
    for i = 1:numel(operations)
        [arm, payload] = operationArm(operations{i}, i);
        [required, deferred] = operationCapabilities(arm, payload, deferred);
        for j = 1:numel(required)
            connection.require(required{j});
            requested{end+1} = required{j};
        end
    end
    if deferred.actionBody
        connection.require('action_body_statement_authoring');
        requested{end+1} = 'action_body_statement_authoring';
    end
    if deferred.memberModifiers
        connection.require('member_modifiers');
        requested{end+1} = 'member_modifiers';
    end
    if deferred.implicitParameters
        connection.require('implicit_parameters');
        requested{end+1} = 'implicit_parameters';
    end
    capabilities = orderedCapabilities(requested);
    request = struct('modelHash', model.hash, 'operations', {operations}, ...
        'acceptDocuments', true);
    answer = opensysml.call(connection, 'ApplyEdits', request, capabilities);
    if hasText(answer, 'error')
        failure = failureName(fieldOr(answer, 'failure', 'EDIT_FAILURE_UNSPECIFIED'));
        diagnostics = opensysml.internal.decodeDiagnostics(fieldOr(answer, 'diagnostics', {}));
        referring = textCells(fieldOr(answer, 'referringElements', {}));
        referrers = recordCells(fieldOr(answer, 'referrers', {}));
        details = struct('failure', failure, 'diagnostics', {diagnostics}, ...
            'referringElements', {referring}, 'referrers', {referrers});
        opensysml.internal.raise(editIdentifier(failure), char(answer.error), ...
            diagnostics, details);
    end
    content = fieldOr(answer, 'content', '');
    applied = decodeApplied(fieldOr(answer, 'applied', {}));
    documents = decodeDocuments(fieldOr(answer, 'documents', {}));
    result = opensysml.EditResult(content, applied, documents);
end

function [arm, payload] = operationArm(operation, index)
    if ~isstruct(operation) || ~isscalar(operation)
        opensysml.internal.raise('opensysml:argument', sprintf( ...
            'operations{%d} must be a scalar edit-operation struct', index));
    end
    names = fieldnames(operation);
    if numel(names) ~= 1 || ~isstruct(operation.(names{1})) || ...
            ~isscalar(operation.(names{1}))
        opensysml.internal.raise('opensysml:argument', sprintf( ...
            'operations{%d} must contain exactly one edit-operation arm', index));
    end
    arm = names{1};
    payload = operation.(arm);
    known = {'setValue', 'rename', 'addMember', 'delete', 'move', ...
        'addConnection', 'addSatisfy', 'addRequirementConstraint', ...
        'addTransition', 'addImport', 'addDocumentation', 'addVerify', ...
        'addMetadata', 'addSequence', 'addMetadataPrefix', 'addComment', 'addNote'};
    if ~any(strcmp(known, arm))
        opensysml.internal.raise('opensysml:argument', sprintf( ...
            'unknown edit operation %s', arm));
    end
end

function [required, deferred] = operationCapabilities(arm, payload, deferred)
    required = {};
    switch arm
        case {'addMember', 'addConnection', 'addSatisfy', ...
                'addRequirementConstraint', 'addTransition', 'addVerify', ...
                'addMetadata', 'addMetadataPrefix', 'addSequence', ...
                'addImport', 'addDocumentation', 'addComment', 'addNote', ...
                'delete', 'move'}
            required{end+1} = 'authoring';
    end
    switch arm
        case 'addConnection'
            required{end+1} = 'connection_authoring';
        case 'addSatisfy'
            required{end+1} = 'satisfy_authoring';
        case 'addRequirementConstraint'
            required{end+1} = 'requirement_constraint_authoring';
        case 'addTransition'
            required{end+1} = 'transition_authoring';
        case 'addVerify'
            required{end+1} = 'verification_objective_authoring';
        case 'addMetadata'
            required{end+1} = 'metadata_authoring';
        case 'addMetadataPrefix'
            required{end+1} = 'metadata_prefix_authoring';
        case 'addSequence'
            required{end+1} = 'sequence_authoring';
            if needsActionBody(payload)
                deferred.actionBody = true;
            end
        case 'addImport'
            required{end+1} = 'import_authoring';
        case 'addDocumentation'
            required{end+1} = 'documentation_authoring';
        case {'addComment', 'addNote'}
            required{end+1} = 'comment_authoring';
        case 'addMember'
            kind = fieldOr(payload, 'kind', '');
            name = fieldOr(payload, 'name', '');
            if strcmp(kind, 'objective') && isempty(name)
                required{end+1} = 'verification_objective_authoring';
            end
            if ~isempty(fieldOr(payload, 'doc', ''))
                required{end+1} = 'documentation_authoring';
            end
            if ~isempty(fieldOr(payload, 'metadataPrefixes', {}))
                required{end+1} = 'metadata_authoring';
            end
            if logicalField(payload, 'isAbstract') || logicalField(payload, 'isDefault') || ...
                    ~isempty(fieldOr(payload, 'redefines', {})) || ...
                    ~isempty(fieldOr(payload, 'direction', '')) || ...
                    any(strcmp(kind, {'ref', 'return'}))
                deferred.memberModifiers = true;
            end
            if isempty(kind)
                deferred.implicitParameters = true;
            end
            if ~isempty(fieldOr(payload, 'bodyExpression', '')) || ...
                    any(strcmp(kind, {'assert', 'assert not', ...
                    'assert constraint', 'assert not constraint'}))
                required{end+1} = 'constraint_body_authoring';
            end
            if any(strcmp(kind, {'exhibit state', 'exhibit', 'entry action', ...
                    'do action', 'exit action'}))
                required{end+1} = 'state_action_authoring';
            end
    end
    required = unique(required, 'stable');
end

function tf = needsActionBody(item)
    keyword = fieldOr(item, 'keyword', '');
    kind = fieldOr(item, 'memberKind', '');
    tf = any(strcmp(keyword, {'if', 'else'})) || ...
        (isempty(keyword) && ~isempty(kind)) || ...
        any(strcmp(kind, {'accept', 'send', 'assign', 'if', 'while', ...
        'loop', 'for', 'terminate'}));
    details = {'condition', 'value', 'target', 'via', 'until', ...
        'body', 'elseBody', 'multiplicity', 'parameter'};
    if any(ismember(details, fieldnames(item))), tf = true; end
end

function capabilities = orderedCapabilities(requested)
    order = {'apply_edits', 'authoring', 'connection_authoring', ...
        'satisfy_authoring', 'requirement_constraint_authoring', ...
        'transition_authoring', 'verification_objective_authoring', ...
        'metadata_authoring', 'metadata_prefix_authoring', ...
        'sequence_authoring', 'action_body_statement_authoring', ...
        'import_authoring', 'documentation_authoring', 'comment_authoring', ...
        'member_modifiers', 'implicit_parameters', ...
        'constraint_body_authoring', 'state_action_authoring', 'edit_documents'};
    capabilities = {};
    for i = 1:numel(order)
        if any(strcmp(requested, order{i}))
            capabilities{end+1} = order{i};
        end
    end
end

function name = failureName(raw)
    if ischar(raw) || (isstring(raw) && isscalar(raw))
        name = char(raw);
        if ~isempty(regexp(name, '^\d+$', 'once'))
            raw = str2double(name);
        else
            return;
        end
    end
    if isnumeric(raw) && isscalar(raw)
        names = {'EDIT_FAILURE_UNSPECIFIED', 'EDIT_FAILURE_NO_OPERATIONS', ...
            'EDIT_FAILURE_UNKNOWN_TARGET', 'EDIT_FAILURE_AMBIGUOUS_TARGET', ...
            'EDIT_FAILURE_NOT_VALUED', 'EDIT_FAILURE_INVALID_VALUE', ...
            'EDIT_FAILURE_INVALID_NAME', 'EDIT_FAILURE_NOT_NAMED', ...
            'EDIT_FAILURE_RENAME_REFERENCED', 'EDIT_FAILURE_OVERLAPPING_EDITS', ...
            'EDIT_FAILURE_RESULT_INVALID', 'EDIT_FAILURE_OWNER_UNKNOWN', ...
            'EDIT_FAILURE_OWNER_NOT_NAMESPACE', 'EDIT_FAILURE_ILLEGAL_KIND', ...
            'EDIT_FAILURE_MEMBER_NAME_TAKEN', 'EDIT_FAILURE_DELETE_REFERENCED', ...
            'EDIT_FAILURE_OWNER_INSIDE_TARGET', 'EDIT_FAILURE_MOVE_REFERENCED', ...
            'EDIT_FAILURE_REFERENCED_ELSEWHERE'};
        index = double(raw) + 1;
        if index >= 1 && index <= numel(names) && index == fix(index)
            name = names{index};
        else
            name = sprintf('EDIT_FAILURE_%d', double(raw));
        end
    else
        name = 'EDIT_FAILURE_UNSPECIFIED';
    end
end

function identifier = editIdentifier(failure)
    token = regexprep(failure, '^EDIT_FAILURE_', '');
    parts = regexp(lower(token), '_', 'split');
    suffix = parts{1};
    for i = 2:numel(parts)
        if ~isempty(parts{i})
            suffix = [suffix upper(parts{i}(1)) parts{i}(2:end)];
        end
    end
    if isempty(token)
        suffix = 'unspecified';
    elseif isempty(regexp(token, '^[A-Za-z0-9_]+$', 'once'))
        suffix = 'failure0';
    end
    if strncmp(token, 'EDIT_FAILURE_', 13) && ...
            ~isempty(regexp(token(14:end), '^\d+$', 'once'))
        suffix = ['failure' token(14:end)];
    end
    identifier = ['opensysml:diagnostics:edit:' suffix];
end

function values = decodeApplied(raw)
    records = recordCells(raw);
    values = cell(1, numel(records));
    for i = 1:numel(records)
        item = records{i};
        values{i} = struct('operationIndex', numberField(item, 'operationIndex', 0), ...
            'target', fieldOr(item, 'target', ''), ...
            'offset', numberField(item, 'offset', 0), ...
            'length', numberField(item, 'length', 0), ...
            'oldText', fieldOr(item, 'oldText', ''), ...
            'newText', fieldOr(item, 'newText', ''), ...
            'document', fieldOr(item, 'document', ''));
    end
end

function values = decodeDocuments(raw)
    records = recordCells(raw);
    values = cell(1, numel(records));
    for i = 1:numel(records)
        values{i} = struct('name', fieldOr(records{i}, 'name', ''), ...
            'content', fieldOr(records{i}, 'content', ''));
    end
end

function values = recordCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {};
    end
end

function values = textCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif ischar(raw), values = cellstr(raw);
    else, values = {};
    end
    for i = 1:numel(values), values{i} = char(values{i}); end
end

function value = fieldOr(source, name, fallback)
    value = fallback;
    if isstruct(source) && isfield(source, name), value = source.(name); end
end

function tf = hasText(source, name)
    tf = isstruct(source) && isfield(source, name) && ~isempty(source.(name));
end

function tf = logicalField(source, name)
    tf = false;
    if isstruct(source) && isfield(source, name)
        tf = logical(source.(name));
    end
end

function value = numberField(source, name, fallback)
    value = fieldOr(source, name, fallback);
    if ischar(value) || (isstring(value) && isscalar(value))
        value = str2double(char(value));
    end
    if isempty(value) || ~isnumeric(value), value = fallback; end
end
