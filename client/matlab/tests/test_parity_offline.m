function test_parity_offline()
%TEST_PARITY_OFFLINE Check local validation and response decoding.

    capabilities = {'verification', 'verification_questions', 'engines', ...
        'schedule', 'schedule_explore', 'performer', 'complex_values', ...
        'structured_values', 'measurement_refs', 'function_values', ...
        'set_values', 'tensor_values', 'metaobject_values', 'infinity_value', ...
        'feature_values'};
    conn = opensysml.external('127.0.0.1:1');
    conn.primeServerInfo(struct('version', 'test', 'capabilities', {capabilities}));
    model = opensysml.Model(conn, 'offline-hash', {});

    quantity = struct('magnitude', int64(3), 'unit', 'kg');
    wire = opensysml.encodeValue(quantity, conn);
    assert_equal(isfield(wire, 'quantity'), true, 'quantity arm');
    decoded = opensysml.decodeValue(wire);
    assert_equal(decoded.magnitude, int64(3), 'quantity round trip');
    assert_error(@() opensysml.encodeValue(struct( ...
        'magnitude', intmax('uint64'), 'unit', 'kg'), conn), ...
        'opensysml:encode', 'quantity integer overflow');

    enumeration = struct('literalId', 'A::b', 'enumerationId', 'A', ...
        'name', 'b', 'value', []);
    wire = opensysml.encodeValue(enumeration, conn);
    assert_equal(isfield(wire, 'enumLiteral'), true, 'enum arm');
    decoded = opensysml.decodeValue(wire);
    assert_equal(decoded.name, 'b', 'enum round trip');

    wire = opensysml.encodeValue(struct('infinity', true), conn);
    decoded = opensysml.decodeValue(wire);
    assert_equal(decoded.infinity, true, 'infinity arm');
    wire = opensysml.encodeValue(struct('calcId', 'A::f', 'self', []), conn);
    decoded = opensysml.decodeValue(wire);
    assert_equal(decoded.calcId, 'A::f', 'function arm');
    wire = opensysml.encodeValue(struct('set', {{int64(1), 'x'}}), conn);
    decoded = opensysml.decodeValue(wire);
    assert_equal(numel(decoded.set), 2, 'set arm');
    wire = opensysml.encodeValue(struct('elementId', 'A::x', 'metaclassId', 'Part'), conn);
    decoded = opensysml.decodeValue(wire);
    assert_equal(decoded.elementId, 'A::x', 'metaobject arm');

    measurement = struct('unit', 'kg', 'unitId', '', 'unitTerm', struct());
    wire = opensysml.encodeValue(measurement, conn);
    assert_equal(isfield(wire, 'measurementRef'), true, 'measurement reference arm');
    decoded = opensysml.decodeValue(wire);
    assert_equal(decoded.unit, 'kg', 'measurement reference round trip');

    arrayValue = struct('dimensions', int64([2 1]), 'elements', {{int64(1), int64(2)}});
    wire = opensysml.encodeValue(arrayValue, conn);
    assert_equal(isfield(wire, 'array'), true, 'array arm');
    decoded = opensysml.decodeValue(wire);
    assert_equal(numel(decoded.elements), 2, 'array round trip');
    wire = opensysml.encodeValue(struct('components', {{1, 2}}), conn);
    assert_equal(isfield(wire, 'vector'), true, 'vector arm');
    vectorQuantity = struct('components', {{quantity, quantity}});
    wire = opensysml.encodeValue(vectorQuantity, conn);
    assert_equal(isfield(wire, 'vectorQuantity'), true, 'vector quantity arm');
    tensor = struct('dimensions', int64(2), 'components', {{quantity, quantity}});
    wire = opensysml.encodeValue(tensor, conn);
    assert_equal(isfield(wire, 'tensorQuantity'), true, 'tensor quantity arm');
    assert_equal(opensysml.parseUint64('18446744073709551615'), ...
        intmax('uint64'), 'exact uint64 parsing');

    instances = containers.Map('KeyType', 'char', 'ValueType', 'any');
    instances('5') = struct('id', int64(5));
    resolved = opensysml.decodeValue(struct('instanceId', '5'), ...
        opensysml.internal.instanceResolver(instances));
    assert_equal(resolved.id, int64(5), 'instance reference resolver');
    decodedInstances = opensysml.internal.decodeInstances(conn, ...
        struct('id', '6', 'featureValues', struct('failed', struct('error', 'failure'))));
    featureError = decodedInstances{1}.feature_values('failed');
    assert_equal(featureError.identifier, 'opensysml:featureValue', 'feature value error');

    fileDocument = opensysml.SourceDocument.file('model.sysml');
    inlineDocument = opensysml.SourceDocument.inline('inline.sysml', 'package Inline;');
    assert_equal(fileDocument.documentName(), 'model.sysml', 'file document name');
    assert_equal(inlineDocument.documentName(), 'inline.sysml', 'inline document name');
    assert_error(@() opensysml.SourceDocument(), 'opensysml:argument', 'empty source document');
    assert_error(@() opensysml.SourceDocument('a.sysml', 'package A;'), ...
        'opensysml:argument', 'file and content document');
    assert_error(@() opensysml.SourceDocument.inline('a.sysml', 'package A;', 'java'), ...
        'opensysml:argument', 'invalid source language');
    assert_error(@() opensysml.parseSources(conn, {}), ...
        'opensysml:argument', 'empty parseSources');
    assert_error(@() opensysml.parseSources(conn, {}, 'language', 'invalid'), ...
        'opensysml:argument', 'invalid parseSources language');
    assert_error(@() opensysml.parseSources(conn, ...
        {{'same.sysml', 'package A;'}, {'same.sysml', 'package B;'}}), ...
        'opensysml:argument', 'duplicate parseSources names');
    assert_error(@() opensysml.parseSources(conn, {}, 'language', 'sysml'), ...
        'opensysml:argument', 'legacy parseSources language option');
    errorInfo = opensysml.lastError();
    assert_equal(~isempty(strfind(errorInfo.message, 'parseSources needs a non-empty cell')), ...
        true, 'parseSources language accepted');

    query = opensysml.buildQuery([], 'scope', {'Demo::vehicle'}, ...
        'select', {'name'}, 'where', struct('property', 'mass', ...
        'operator', '>', 'value', 1500));
    assert_equal(query.scope, {'Demo::vehicle'}, 'structured query scope');
    assert_equal(query.select, {'name'}, 'structured query selection');
    assert_equal(query.where.primitive.operator, 'PRIMITIVE_OPERATOR_GREATER', ...
        'structured query operator');
    assert_equal(query.where.primitive.value, {'1500.0'}, 'structured query comparison');
    assert_error(@() opensysml.buildQuery([], 'where', ...
        struct('property', 'mass', 'operator', '?', 'value', 1)), ...
        'opensysml:argument', 'invalid structured query');
    assert_error(@() opensysml.buildQuery(struct('type', 'Element')), ...
        'opensysml:argument', 'invalid query type');

    bindings = struct();
    bindings.root = {struct('type', 'element', 'id', 'Demo::vehicle')};
    bindings.limit = {int64(3), 4.5, true};
    wireBindings = opensysml.buildDocumentBindings(bindings);
    assert_equal(wireBindings{1}.parameter, 'root', 'document binding name');
    assert_equal(wireBindings{1}.values{1}.elementId, 'Demo::vehicle', ...
        'element binding');
    assert_equal(wireBindings{2}.values{1}.intValue, '3', 'integer binding');
    assert_equal(wireBindings{2}.values{2}.realValue, 4.5, 'real binding');
    assert_equal(wireBindings{2}.values{3}.boolValue, true, 'boolean binding');
    assert_error(@() opensysml.buildDocumentBindings(struct('bad', {struct('type', 'verdict')})), ...
        'opensysml:argument', 'unsupported document binding');

    verdictValues = opensysml.internal.decodeVerdicts(struct('kind', 'constraint', ...
        'elementId', 'Demo::massOK', 'holds', false, ...
        'condition', 'mass < 100.0'), [], {}, {});
    verdict = verdictValues{1};
    assert_equal(~isempty(strfind(verdict.explain(), 'mass < 100.0')), ...
        true, 'verdict explanation');
    exploration = opensysml.internal.decodeExplorationResponse(struct( ...
        'exploration', struct('complete', false, 'runs', 2, ...
        'budgetsHit', {{'runs'}}, 'runsBudget', 5, 'depthBudget', 9, ...
        'probabilitiesLowerBound', true), 'outcomes', {{}}), conn);
    assert_equal(exploration.status(), ...
        'incomplete: runs budget 5 hit after 2 runs; probabilities are lower bounds', ...
        'exploration status');
    standing = opensysml.internal.decodeStanding(struct('engine', 'solver', ...
        'strength', 'proved', 'bounds', struct('name', 'runs', ...
        'limit', '8', 'reached', true)));
    assert_equal(standing.explain(), 'proved by solver (runs 8 reached)', ...
        'standing explanation');
    engine = opensysml.EngineInfo(struct('name', 'solver', 'authority', 'local', ...
        'answers', {{'constraint'}}, 'ready', true));
    assert_equal(~isempty(strfind(engine.explain(), 'solver: local')), true, ...
        'engine explanation');
    summaryVerdict = opensysml.internal.decodeVerdicts( ...
        struct('kind', 'validation', 'holds', true), [], {}, {});
    validation = opensysml.Validation({}, summaryVerdict{1}, [], {}, {}, false);
    assert_equal(validation.valid(), true, 'validation summary');
    assert_equal(logical(validation), true, 'validation logical value');

    assert_error(@() opensysml.convert(conn, 'sysml'), ...
        'opensysml:argument', 'convert without a source');
    assert_error(@() opensysml.convert(conn, 'sysml', ...
        'content', 'package A;', 'modelHash', 'also-a-source'), ...
        'opensysml:argument', 'convert with multiple sources');
    assert_error(@() opensysml.runSweep(model, 'Demo::calc', struct('x', {{1}})), ...
        'opensysml:argument', 'invalid sweep range');

    assert_error(@() opensysml.executeAction(model, 'Demo::action', ...
        'schedule', 'explore'), 'opensysml:argument', 'executeAction exploration schedule');
    last = opensysml.lastError();
    assert_equal(~isempty(strfind(last.message, 'explore_action')), true, ...
        'executeAction exploration remedy');
    assert_error(@() opensysml.executeState(model, 'Demo::Machine', ...
        'schedule', 'explore'), 'opensysml:argument', 'executeState exploration schedule');
    assert_error(@() opensysml.exploreAction(model, 'Demo::action', ...
        'schedule', 'declared'), 'opensysml:argument', 'exploreAction schedule');
    assert_error(@() opensysml.exploreState(model, 'Demo::Machine', ...
        'schedule', 'declared'), 'opensysml:argument', 'exploreState schedule');
    assert_error(@() opensysml.runAnalysis(model, 'Demo::analysis', ...
        'schedule', 'explore'), 'opensysml:argument', 'runAnalysis exploration schedule');
    assert_error(@() opensysml.exploreAnalysis(model, 'Demo::analysis', ...
        'schedule', 'declared'), 'opensysml:argument', 'exploreAnalysis schedule');
    fprintf('parity offline ok\n');
end
