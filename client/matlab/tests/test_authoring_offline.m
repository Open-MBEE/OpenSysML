function test_authoring_offline()
%TEST_AUTHORING_OFFLINE Check edit wire shapes and local validation.

    model = opensysml.Model([], 'offline-hash', {}, {}, ...
        {'first.sysml', 'second.sysml'}, '');
    editor = model.edit();
    assert_equal(isa(editor, 'opensysml.Editor'), true, 'Model.edit');
    assert_equal(editor.numOperations(), 0, 'empty editor');

    editor.setValue('Demo::mass', '3.0');
    editor.rename('Demo::mass', 'weight');
    editor.addMember('Demo', 'part def', 'Widget', 'type', 'Base', ...
        'metadata', {'Q::Tag'}, 'doc', 'A widget.');
    editor.addObjective('Demo');
    editor.addVerify('Demo::Test', 'Demo::Requirement');
    editor.addMetadata('Demo', 'Q::Safety', 'values', {{'level', 'high'}});
    editor.addMetadataPrefix('Demo::Widget', 'Q::Tag');
    editor.addDocumentation('Demo::Widget', 'Widget docs', ...
        'name', 'Summary', 'locale', 'en', 'replace', true);
    editor.addComment('Demo', 'A comment', 'about', {'Demo::Widget'});
    editor.addNote('Demo::Widget', 'A note');
    editor.addSatisfy('Demo', 'Demo::Requirement', 'by', 'Demo::Widget');
    editor.addRequireConstraint('Demo::Requirement', 'x > 0', 'name', 'Positive');
    editor.addAssumeConstraint('Demo::Requirement', 'x < 10');
    editor.addTransition('Demo::Machine', 'Demo::Idle', 'Demo::Ready', ...
        'name', 'start', 'trigger', 'start', 'guard', 'ready', 'effect', 'begin');
    editor.addEntryTransition('Demo::Machine', 'Demo::Idle');
    editor.addFirst('Demo::run', 'start');
    editor.addThen('Demo::run', 'ref', 'finish');
    editor.addAccept('Demo::run', 'payload', 'type', 'Payload');
    editor.addSend('Demo::run', 'payload', 'to', 'receiver', 'via', 'port');
    editor.addAssign('Demo::run', 'count', 'count + 1');
    nested = opensysml.Body();
    nested.addAssign('count', 'count + 1');
    editor.addIf('Demo::run', 'ready', nested);
    editor.addWhile('Demo::run', 'ready', nested, 'until', 'done');
    editor.addLoop('Demo::run', nested, 'until', 'done');
    editor.addFor('Demo::run', 'item', 'items', nested, 'type', 'Item');
    editor.addTerminate('Demo::run', 'occurrence', 'done');
    editor.addGuardedThen('Demo::run', 'ready', 'finish');
    editor.addElse('Demo::run', 'fallback');
    editor.addImport('Demo', 'ScalarValues::*', ...
        'visibility', 'private', 'recursive', true, 'all', true, ...
        'filter', {'@Q::Safety'});
    editor.addConnection('Demo', 'connection', 'left', 'right', ...
        'name', 'link', 'type', 'LinkType');
    editor.addAllocation('Demo', 'source', 'target', 'name', 'allocated');
    editor.addFlow('Demo', 'source', 'target', 'name', 'flowed');
    editor.addSuccession('Demo', 'source', 'target', 'name', 'next');
    editor.deleteElement('Demo::Old', 'cascade', true);
    editor.move('Demo::Widget', 'Demo::Other');
    editor.addPackage('Demo', 'P');
    editor.addPartDef('Demo', 'Part');
    editor.addPart('Demo', 'partUsage', 'type', 'Part');
    editor.addAttributeDef('Demo', 'Attribute');
    editor.addAttribute('Demo', 'value', 'type', 'ScalarValues::Integer');
    editor.addItemDef('Demo', 'Item');
    editor.addItem('Demo', 'item');
    editor.addPortDef('Demo', 'Port');
    editor.addPort('Demo', 'port');
    editor.addClass('Demo', 'Class');
    editor.addStruct('Demo', 'Struct');
    editor.addDatatype('Demo', 'Data');
    editor.addClassifier('Demo', 'Classifier');
    editor.addFeature('Demo', 'feature');
    editor.addAssoc('Demo', 'Association');
    editor.addBehavior('Demo', 'Behavior');
    editor.addFunction('Demo', 'function');
    editor.addPredicate('Demo', 'predicate');
    editor.addInteraction('Demo', 'Interaction');
    editor.addMetaclass('Demo', 'Metaclass');
    editor.addCalcDef('Demo', 'calcDef', 'inputs', {{'x', 'ScalarValues::Integer'}}, ...
        'returnType', 'ScalarValues::Integer', 'returnExpression', 'x');
    editor.addCalc('Demo', 'calc', 'expression', '2 + 2');
    editor.addParameter('Demo::calc', 'in', 'x', 'type', 'ScalarValues::Integer');
    editor.addReturn('Demo::calc', 'type', 'ScalarValues::Integer', 'value', 'x');
    editor.addActionDef('Demo', 'actionDef', 'inputs', {{'x', 'Item'}});
    editor.addAction('Demo', 'action');
    editor.addPerformAction('Demo', 'performed');
    editor.addPerform('Demo', 'Demo::performed');
    editor.addExhibitState('Demo', 'state');
    editor.addExhibit('Demo', 'Demo::state');
    editor.addStateAction('Demo::state', 'entry', 'onEntry');
    editor.addStateDef('Demo', 'State');
    editor.addState('Demo', 'stateUsage', 'type', 'State');
    editor.addConstraintDef('Demo', 'Constraint', 'expression', 'x > 0');
    editor.addConstraint('Demo', 'constraint', 'expression', 'x > 0');
    editor.addAssertConstraint('Demo', 'expression', 'x > 0');
    editor.addAssert('Demo', 'Demo::constraint');
    editor.addRequirementDef('Demo', 'Requirement');
    editor.addRequirement('Demo', 'requirement', 'type', 'Requirement');

    operations = editor.operations();
    assert_equal(iscell(operations), true, 'operations use a cell');
    assert_equal(editor.numOperations(), numel(operations), 'operation count');
    for i = 1:numel(operations)
        arms = fieldnames(operations{i});
        assert_equal(numel(arms), 1, 'operation has one oneof arm');
        assert_equal(isstruct(operations{i}.(arms{1})), true, ...
            'operation arm is a struct');
    end
    expectedArms = {'setValue', 'rename', 'addMember', 'addVerify', ...
        'addMetadata', 'addMetadataPrefix', 'addDocumentation', 'addComment', ...
        'addNote', 'addSatisfy', 'addRequirementConstraint', 'addTransition', ...
        'addSequence', 'addImport', 'addConnection', 'delete', 'move'};
    actualArms = cellfun(@(op) fieldnames(op), operations, 'UniformOutput', false);
    actualArms = cellfun(@(names) names{1}, actualArms, 'UniformOutput', false);
    for i = 1:numel(expectedArms)
        assert_equal(any(strcmp(actualArms, expectedArms{i})), true, ...
            ['operation arm ' expectedArms{i}]);
    end

    addMember = operationWithArm(operations, 'addMember');
    assert_equal(isfield(addMember.addMember, 'metadataPrefixes'), true, ...
        'addMember lowerCamel repeated field');
    assert_equal(iscell(addMember.addMember.specializes), true, ...
        'addMember repeated references use a cell');
    addImport = operationWithArm(operations, 'addImport');
    assert_equal(isfield(addImport.addImport, 'isRecursive'), true, ...
        'addImport lowerCamel field');
    assert_equal(iscell(addImport.addImport.filters), true, ...
        'addImport filters use a cell');
    addSequence = operationWithArmField(operations, 'addSequence', 'body');
    assert_equal(isfield(addSequence.addSequence, 'memberKind'), true, ...
        'sequence lowerCamel field');
    assert_equal(iscell(addSequence.addSequence.body), true, ...
        'nested action body uses a cell');

    body = opensysml.Body();
    body.addFirst('start');
    body.addThen('next');
    body.addThen([], 'action', 'created', 'type', 'Action');
    body.addAction('plain', 'type', 'Action');
    body.addAccept('payload', 'type', 'Payload', 'via', 'port');
    body.addSend('payload', 'to', 'receiver', 'via', 'port');
    body.addAssign('target', 'value');
    body.addIf('ready', nested, 'elseBody', nested);
    body.addWhile('ready', nested, 'until', 'done');
    body.addLoop(nested, 'until', 'done');
    body.addFor('item', 'items', nested, 'type', 'Item');
    body.addTerminate('occurrence');
    body.addGuardedThen('ready', 'next');
    body.addElse('fallback');
    statements = body.operations();
    assert_equal(numel(statements), 14, 'all Body statements represented');
    assert_equal(statements{1}.keyword, 'first', 'Body first statement');
    assert_equal(statements{2}.keyword, 'then', 'Body then statement');
    assert_equal(statements{3}.memberName, 'created', 'Body then action form');
    assert_equal(statements{8}.elseBody{1}.memberKind, 'assign', ...
        'nested if else body');
    assert_equal(statements{9}.until, 'done', 'while until field');
    assert_equal(statements{11}.parameter, 'item', 'for parameter field');
    assert_equal(statements{12}.memberKind, 'terminate', 'terminate statement');

    expect_error(@() opensysml.Editor([]), 'opensysml:argument');
    emptyEditor = model.edit();
    expect_error(@() emptyEditor.apply(), ...
        'opensysml:diagnostics:edit:noOperations');
    err = opensysml.lastError();
    assert_equal(err.details.failure, 'EDIT_FAILURE_NO_OPERATIONS', ...
        'empty edit failure details');
    expect_error(@() opensysml.applyEdits(model, struct()), 'opensysml:argument');
    operationConnection = opensysml.external('127.0.0.1:1');
    operationConnection.primeServerInfo(struct('version', 'test', ...
        'capabilities', {{'apply_edits'}}));
    operationModel = opensysml.Model(operationConnection, 'offline-hash', ...
        {}, {}, {'single.sysml'}, '');
    invalidOperation = struct('setValue', struct(), 'rename', struct());
    expect_error(@() opensysml.applyEdits(operationModel, {invalidOperation}), ...
        'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.setValue('Demo::x', 1), ...
        'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addMember('Demo', 'part', ...
        'x', 'abstract', 1), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addSatisfy('Demo', 'Req', ...
        'asserted', 1), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addMetadata('Demo', 'Tag', ...
        'values', {{'feature'}}), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addThen('Demo', 'ref', 'x', ...
        'action', 'y'), 'opensysml:argument');
    validationBody = opensysml.Body();
    expect_error(@() validationBody.addThen('ref', 'action', 'x'), ...
        'opensysml:argument');
    validationBody = opensysml.Body();
    expect_error(@() validationBody.addAccept('payload', ...
        'then', false, 'multiplicity', '[1]'), 'opensysml:argument');
    validationBody = opensysml.Body();
    expect_error(@() validationBody.addIf('true', []), 'opensysml:argument');
    validationBody = opensysml.Body();
    expect_error(@() validationBody.addAssign('target', 'value', 'then', 1), ...
        'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addNote('Demo::x', sprintf('two\nlines')), ...
        'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addMember('Demo', 'part', 'x', ...
        'direction', 1), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addMember('Demo', 'part', 'x', ...
        'redefines', {'Base::part', 1}), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addImport('Demo', 3), ...
        'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addImport('Demo', 'A::*', ...
        'recursive', 'yes'), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addConnection('Demo', 'flow', ...
        'from', 2), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addDocumentation('Demo::x', [], ...
        'replace', 'yes'), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addComment('Demo', 'body', ...
        'about', 'Demo::x'), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addCalcDef('Demo', 'Calc', ...
        'inputs', {{'x'}}), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addCalc('Demo', 'Calc', ...
        'returnExpression', 'x'), 'opensysml:argument');
    validationEditor = opensysml.Editor(model);
    expect_error(@() validationEditor.addStateAction('Demo', 'start', 'x'), ...
        'opensysml:argument');

    tooDeep = opensysml.Body();
    for i = 1:129
        wrapped = opensysml.Body();
        wrapped.addIf('true', tooDeep);
        tooDeep = wrapped;
    end
    wrapped = opensysml.Body();
    expect_error(@() wrapped.addIf('true', tooDeep), 'opensysml:argument');

    offlineConnection = opensysml.external('127.0.0.1:1');
    offlineConnection.primeServerInfo(struct('version', 'test', ...
        'capabilities', {{'apply_edits', 'authoring', 'sequence_authoring'}}));
    gatedModel = opensysml.Model(offlineConnection, 'offline-hash', {}, {}, ...
        {'single.sysml'}, '');
    missingBodyCapability = gatedModel.edit();
    missingBodyCapability.addIf('Demo::run', 'true', nested);
    expect_error(@() missingBodyCapability.apply(), 'opensysml:missingCapability');
    err = opensysml.lastError();
    assert_equal(err.details.capability, 'action_body_statement_authoring', ...
        'action body capability gate');
    assert_equal(~isempty(strfind(err.message, 'GetServerInfo reports')), ...
        true, 'capability remedy');

    gatedConnection = opensysml.external('127.0.0.1:1');
    gatedConnection.primeServerInfo(struct('version', 'test', ...
        'capabilities', {{'apply_edits', 'authoring'}}));
    gatedModel = opensysml.Model(gatedConnection, 'offline-hash', {}, {}, ...
        {'single.sysml'}, '');
    missingModifiers = gatedModel.edit();
    missingModifiers.addMember('Demo', 'part', 'x', 'abstract', true);
    expect_error(@() missingModifiers.apply(), 'opensysml:missingCapability');
    err = opensysml.lastError();
    assert_equal(err.details.capability, 'member_modifiers', ...
        'member modifier capability gate');

    fprintf('authoring offline ok\n');
end

function operation = operationWithArm(operations, arm)
    operation = [];
    for i = 1:numel(operations)
        if isfield(operations{i}, arm)
            operation = operations{i};
            return;
        end
    end
    error('assert:error', 'missing operation arm %s', arm);
end

function operation = operationWithArmField(operations, arm, field)
    operation = [];
    for i = 1:numel(operations)
        if isfield(operations{i}, arm) && isfield(operations{i}.(arm), field)
            operation = operations{i};
            return;
        end
    end
    error('assert:error', 'missing operation arm %s field %s', arm, field);
end

function expect_error(fn, identifier)
    try
        fn();
    catch e
        if strcmp(e.identifier, identifier), return; end
        error('assert:error', 'got %s, want %s', e.identifier, identifier);
    end
    error('assert:error', 'expected %s', identifier);
end
