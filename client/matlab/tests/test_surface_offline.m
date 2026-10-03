function test_surface_offline()
%TEST_SURFACE_OFFLINE Check local validation and response decoding.

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
    wide = struct('bigInteger', '1180591620717411303424');
    wideQuantity = struct('magnitude', wide, 'unit', 'kg');
    for value = {wide, {int64(1), {wide}}, struct('set', {{wide}}), wideQuantity, ...
            struct('components', {{wideQuantity}}), ...
            struct('dimensions', int64(1), 'components', {{wideQuantity}})}
        assert_error(@() opensysml.encodeValue(value{1}, conn), ...
            'opensysml:missingCapability', 'Integer beyond int64 without big_int_values');
    end
    bigConn = opensysml.external('127.0.0.1:1');
    bigConn.primeServerInfo(struct('version', 'test', 'capabilities', ...
        {[capabilities, {'big_int_values'}]}));
    wire = opensysml.encodeValue({int64(1), {wide}}, bigConn);
    assert_equal(wire.sequence.elements{2}.sequence.elements{1}.bigIntValue, ...
        '1180591620717411303424', 'nested Integer beyond int64 with big_int_values');
    third = struct('numerator', '1', 'denominator', '3');
    thirdQuantity = struct('magnitude', third, 'unit', 'kg');
    for value = {third, {int64(1), {third}}, thirdQuantity, struct('components', {{thirdQuantity}})}
        assert_error(@() opensysml.encodeValue(value{1}, conn), ...
            'opensysml:missingCapability', 'exact Rational without rational_values');
    end
    rationalConn = opensysml.external('127.0.0.1:1');
    rationalConn.primeServerInfo(struct('version', 'test', 'capabilities', ...
        {[capabilities, {'rational_values'}]}));
    wire = opensysml.encodeValue({int64(1), {third}}, rationalConn);
    assert_equal(wire.sequence.elements{2}.sequence.elements{1}.rationalValue, third, ...
        'nested exact Rational with rational_values');
    half = struct('numerator', '1', 'denominator', '2');
    assert_equal(opensysml.encodeValue(half, rationalConn), struct('rationalValue', half), ...
        'double-exact Rational with rational_values');
    wire = opensysml.encodeValue({half}, conn);
    assert_equal(wire.sequence.elements{1}, struct('realValue', 0.5), ...
        'double-exact Rational without rational_values');
    halfQuantity = opensysml.encodeValue(struct('magnitude', half, 'unit', 'kg'), conn);
    assert_equal(halfQuantity.quantity.realMagnitude, 0.5, ...
        'double-exact Rational magnitude without rational_values');
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
    parent = struct('id', '1', 'typeSymbolId', 'Demo::Parent', ...
        'featureValues', struct('child', struct('value', struct('instanceId', '2')), ...
        'missing', struct('value', struct('instanceId', '99'))));
    child = struct('id', '2', 'typeSymbolId', 'Demo::Child', ...
        'featureValues', struct());
    [graphValues, graph] = opensysml.internal.decodeInstances(conn, {parent, child});
    resolvedChild = graphValues{1}.feature_values('child');
    unresolvedChild = graphValues{1}.feature_values('missing');
    assert_equal(resolvedChild.id, int64(2), 'instance graph child reference');
    assert_equal(unresolvedChild, int64(99), 'missing instance graph reference');
    assert_equal(graph('1').id, int64(1), 'instance graph index');
    cycleA = struct('id', '1', 'typeSymbolId', 'Demo::A', ...
        'featureValues', struct('next', struct('value', struct('instanceId', '2'))));
    cycleB = struct('id', '2', 'typeSymbolId', 'Demo::B', ...
        'featureValues', struct('next', struct('value', struct('instanceId', '1'))));
    [cycleValues, ~] = opensysml.internal.decodeInstances(conn, {cycleA, cycleB});
    next = cycleValues{1}.feature_values('next');
    assert_equal(next.id, int64(2), 'cycle child instance');
    assert_equal(next.feature_values('next'), int64(1), 'cycle reference terminates');

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
    rebuiltQuery = opensysml.buildQuery(query);
    assert_equal(isequal(rebuiltQuery, query), true, 'built query is idempotent');
    assert_equal(isequal(opensysml.buildQuery([], 'where', query.where), ...
        opensysml.buildQuery([], 'where', struct('property', 'mass', ...
        'operator', '>', 'value', 1500))), true, 'built primitive constraint');
    composite = struct('type', 'CompositeConstraint', 'operator', 'and', ...
        'constraint', {{query.where, struct('property', 'name', ...
        'operator', '=', 'value', 'vehicle')}});
    compositeQuery = opensysml.buildQuery([], 'where', composite);
    assert_equal(isequal(opensysml.buildQuery(compositeQuery), compositeQuery), ...
        true, 'built composite constraint is idempotent');
    assert_error(@() opensysml.buildQuery([], 'where', ...
        struct('property', 'mass', 'operator', '?', 'value', 1)), ...
        'opensysml:argument', 'invalid structured query');
    assert_error(@() opensysml.buildQuery(struct('type', 'Element')), ...
        'opensysml:argument', 'invalid query type');

    documentObject = struct('instanceId', '17', 'path', '#17', ...
        'element', struct('elementId', 'Demo::part', 'elementType', 'PartUsage'));
    objectRow = struct('element', struct('object', documentObject));
    verdictValue = struct('assertion', struct('elementId', 'Demo::check', ...
        'elementType', 'Constraint'), 'kind', 'constraint', ...
        'text', 'assert constraint check', 'path', '#17', ...
        'verdict', 'holds', 'condition', '', 'reason', '', 'verification', {{}});
    verdictRow = struct('element', struct('verdict', verdictValue));
    stateValue = struct('object', documentObject, 'machine', 'Demo::Machine', ...
        'name', 'Running', 'statePath', 'Running', ...
        'state', struct('elementId', 'Demo::Running', 'elementType', 'StateUsage'), ...
        'region', '', 'enclosing', {{}});
    stateRow = struct('element', struct('state', stateValue));
    eventValue = struct('kind', 'transition', 'time', struct('realValue', 1), ...
        'object', documentObject, 'machine', 'Demo::Machine', 'state', 'Running', ...
        'from', 'Idle', 'to', 'Running', 'target', struct(), 'event', '', ...
        'payload', {{}}, 'alternatives', {{}}, 'taken', '');
    eventRow = struct('element', struct('event', eventValue));
    documentResult = opensysml.internal.decodeDocumentResult( ...
        struct('rows', {{objectRow, verdictRow, stateRow, eventRow}}));
    documentRows = documentResult.rows;
    assert_equal(documentRows{1}.object.type, 'object', 'document object row');
    assert_equal(documentRows{1}.element.id, 'Demo::part', 'object row element');
    assert_equal(documentRows{2}.verdict.type, 'verdict', 'document verdict row');
    assert_equal(documentRows{2}.element.id, 'Demo::check', 'verdict row element');
    assert_equal(documentRows{3}.state.type, 'state', 'document state row');
    assert_equal(documentRows{3}.object.type, 'object', 'state row object');
    assert_equal(documentRows{3}.element.id, 'Demo::part', 'state row element');
    assert_equal(documentRows{4}.event.type, 'event', 'document event row');
    assert_equal(documentRows{4}.object.type, 'object', 'event row object');
    assert_equal(documentRows{4}.element.id, 'Demo::part', 'event row element');
    decodedState = opensysml.internal.decodeDocumentValue(struct('state', stateValue));
    assert_equal(decodedState.object.type, 'object', 'state value object');

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
    wide = opensysml.buildDocumentBindings(struct('n', {{struct('bigInteger', '9223372036854775808')}}));
    assert_equal(wide{1}.values{1}.bigIntValue, '9223372036854775808', 'Integer binding beyond int64');
    decodedWide = opensysml.internal.decodeDocumentValue(struct('bigIntValue', '-1180591620717411303424'));
    assert_equal(decodedWide.bigInteger, '-1180591620717411303424', 'document Integer beyond int64');
    exact = opensysml.buildDocumentBindings(struct('r', {{struct('numerator', '2', 'denominator', '3')}}));
    assert_equal(exact{1}.values{1}.rationalValue.denominator, '3', 'exact Rational binding');
    decodedExact = opensysml.internal.decodeDocumentValue(struct('rationalValue', ...
        struct('numerator', '-2', 'denominator', '3')));
    assert_equal(decodedExact.numerator, '-2', 'document exact Rational');
    queryConn = opensysml.external('127.0.0.1:1');
    queryConn.primeServerInfo(struct('version', 'test', 'capabilities', ...
        {{'document_query'}}));
    queryModel = opensysml.Model(queryConn, 'offline-hash', {});
    wideBound = struct('bigInteger', '9223372036854775808');
    for bound = {wideBound, struct('magnitude', wideBound, 'unit', '')}
        assert_error(@() opensysml.runDocumentQuery(queryModel, 'Q::q', ...
            'bindings', struct('n', {bound})), ...
            'opensysml:missingCapability', 'document binding beyond int64 without big_int_values');
    end
    thirdBound = struct('numerator', '1', 'denominator', '3');
    for bound = {thirdBound, struct('magnitude', thirdBound, 'unit', '')}
        assert_error(@() opensysml.runDocumentQuery(queryModel, 'Q::q', ...
            'bindings', struct('n', {bound})), ...
            'opensysml:missingCapability', 'document exact Rational without rational_values');
    end
    halfBound = struct('numerator', '1', 'denominator', '2');
    older = opensysml.buildDocumentBindings( ...
        struct('n', {{halfBound, thirdBound, struct('magnitude', halfBound, 'unit', 'kg')}}));
    older = cellfun(@opensysml.internal.bindingRationalsAsReals, older, 'UniformOutput', false);
    assert_equal(older{1}.values{1}, struct('realValue', 0.5), ...
        'document double-exact Rational as a Real without rational_values');
    assert_equal(older{1}.values{2}.rationalValue, thirdBound, ...
        'document Rational no double holds kept without rational_values');
    assert_equal(older{1}.values{3}.quantity.realMagnitude, 0.5, ...
        'document double-exact Rational magnitude as a Real without rational_values');
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

    decoded = opensysml.internal.decodeJson('{"@id":"a","x":1}');
    assert_equal(isa(decoded, 'containers.Map'), true, 'preserved JSON key map');
    assert_equal(decoded('@id'), 'a', 'preserved @id key');
    assert_equal(decoded('x'), 1, 'preserved map member');
    decoded = opensysml.internal.decodeJson('{"@id"  : "spaced"}');
    assert_equal(decoded('@id'), 'spaced', 'key separator whitespace');
    decoded = opensysml.internal.decodeJson('{"this.isSolid":true}');
    assert_equal(decoded('this.isSolid'), true, 'preserved dotted key');
    unicodeKey = 'μ.member';
    decoded = opensysml.internal.decodeJson(['{"' unicodeKey '":2}']);
    assert_equal(decoded(unicodeKey), 2, 'preserved unicode key');
    decoded = opensysml.internal.decodeJson('{"switch":1}');
    assert_equal(decoded('switch'), 1, 'preserved keyword key');
    longKey = repmat('k', 1, 70);
    decoded = opensysml.internal.decodeJson(['{"' longKey '":3}']);
    assert_equal(decoded(longKey), 3, 'preserved long key');
    decoded = opensysml.internal.decodeJson('{"osk_x_1":"literal"}');
    assert_equal(decoded('osk_x_1'), 'literal', 'preserved escape-prefix key');
    decoded = opensysml.internal.decodeJson('{"@id":"a","xFunction":"literal"}');
    assert_equal(decoded('xFunction'), 'literal', 'preserved xFunction map key');
    decoded = opensysml.internal.decodeJson('{"@id":"a","function":1}');
    assert_equal(isa(decoded, 'containers.Map'), true, 'function map key');
    assert_equal(numel(decoded.keys), 2, 'function and @id map entries');
    assert_equal(decoded('@id'), 'a', 'function map @id value');
    assert_equal(decoded('function'), 1, 'function map value');
    decoded = opensysml.internal.decodeJson('{"function":1,"xFunction":2}');
    assert_equal(isa(decoded, 'containers.Map'), true, 'distinct function map keys');
    assert_equal(decoded('function'), 1, 'function key beside xFunction');
    assert_equal(decoded('xFunction'), 2, 'xFunction key beside function');
    decoded = opensysml.internal.decodeJson( ...
        '{"items":[{"@id":"one","value":1},{"@id":"two","value":2}]}');
    arrayItems = decoded.items;
    if isstruct(arrayItems), arrayItems = num2cell(arrayItems); end
    assert_equal(isa(arrayItems{1}, 'containers.Map'), true, 'escaped object array');
    firstItem = arrayItems{1};
    secondItem = arrayItems{2};
    assert_equal(firstItem('@id'), 'one', 'first escaped array member');
    assert_equal(secondItem('@id'), 'two', 'second escaped array member');
    signedZeros = opensysml.internal.decodeJson( ...
        '{"bare":-0,"decimal":-0.0,"exponent":-0e0,"text":"-0"}');
    assert_equal(1 / signedZeros.bare == -Inf, true, 'bare negative zero');
    assert_equal(1 / signedZeros.decimal == -Inf, true, 'decimal negative zero');
    assert_equal(1 / signedZeros.exponent == -Inf, true, 'exponent negative zero');
    assert_equal(signedZeros.text, '-0', 'negative zero text unchanged');
    one = opensysml.internal.decodeJson('{"one":1e-0}');
    assert_equal(one.one, 1, 'negative exponent zero remains part of number');
    escapedText = 'quoted " text with \ slash';
    decodedText = opensysml.internal.decodeJson(jsonencode( ...
        struct('content', escapedText)));
    assert_equal(decodedText.content, escapedText, 'escaped text token');
    largeText = repmat('x', 1, 12 * 1024 * 1024);
    decodedLarge = opensysml.internal.decodeJson(jsonencode( ...
        struct('content', largeText)));
    assert_equal(numel(decodedLarge.content), numel(largeText), ...
        'large JSON string token');
    functionWire = opensysml.internal.decodeJson('{"function":{"calcId":"C"}}');
    functionKeys = functionWire.keys;
    assert_equal(isa(functionWire, 'containers.Map'), true, ...
        'function Value remains a key map');
    assert_equal(numel(functionKeys), 1, 'single function Value map key');
    assert_equal(functionKeys{1}, 'function', 'function Value map key name');
    decodedFunction = opensysml.decodeValue(functionWire);
    assert_equal(decodedFunction.calcId, 'C', 'reserved function value arm');
    expectedFunction = opensysml.decodeValue( ...
        struct('function', struct('calcId', 'C')));
    assert_equal(decodedFunction, expectedFunction, 'function value via map arm');
    assert_equal(opensysml.internal.decodeValues(functionWire), expectedFunction, ...
        'function value via decodeValues');
    sequenceWire = opensysml.internal.decodeJson( ...
        '{"sequence":{"elements":[{"function":{"calcId":"C"}}]}}');
    decodedSequence = opensysml.internal.decodeValues(sequenceWire);
    assert_equal(decodedSequence{1}, expectedFunction, ...
        'function value nested in sequence');
    namedValuesWire = opensysml.internal.decodeJson( ...
        '{"values":{"this.x":{"function":{"calcId":"C"}}}}');
    decodedNamedValues = opensysml.internal.decodeValues(namedValuesWire);
    namedValueMap = decodedNamedValues.values;
    assert_equal(namedValueMap('this.x'), expectedFunction, ...
        'function value nested in named-values map');
    decoded = opensysml.internal.decodeValues( ...
        opensysml.internal.decodeJson('{"xFunction":1}'));
    assert_equal(isstruct(decoded), true, 'xFunction is ordinary response data');
    assert_equal(decoded.xFunction, 1, 'xFunction data preserved');

    emptyType = opensysml.Symbol(struct('id', 'Demo::Empty', 'typeInfo', struct()), model);
    assert_equal(isstruct(emptyType.typeFacts), true, 'present empty type facts');
    assert_equal(isempty(fieldnames(emptyType.typeFacts)), true, 'empty type facts remain empty');
    decodedMeasurement = opensysml.decodeValue( ...
        struct('measurementRef', struct('unit', 'm/s', 'unitTerm', struct())));
    assert_equal(decodedMeasurement.unitId, '', 'missing measurement reference unit id');
    unavailable = opensysml.EngineInfo(struct('name', 'tool:fmi', ...
        'unavailableReason', 'not installed'));
    assert_equal(unavailable.ready, false, 'missing engine ready default');

    lowerBinding = opensysml.buildDocumentBindings(struct('bound', intmin('int64')));
    upperBinding = opensysml.buildDocumentBindings(struct('bound', intmax('int64')));
    assert_equal(lowerBinding{1}.values{1}.intValue, '-9223372036854775808', ...
        'document binding int64 minimum');
    assert_equal(upperBinding{1}.values{1}.intValue, '9223372036854775807', ...
        'document binding int64 maximum');
    overflow = uint64(intmax('int64')) + uint64(1);
    assert_error(@() opensysml.buildDocumentBindings(struct('bound', overflow)), ...
        'opensysml:argument', 'document binding int64 overflow');
    assert_error(@() opensysml.buildDocumentBindings(struct('bound', ...
        struct('type', 'object', 'id', overflow))), ...
        'opensysml:argument', 'object reference int64 overflow');
    assert_error(@() opensysml.buildDocumentBindings(struct('bound', ...
        struct('type', 'object', 'id', -2^63 - 2048))), ...
        'opensysml:argument', 'object reference int64 underflow');
    minimumWire = opensysml.encodeValue(intmin('int64'));
    maximumWire = opensysml.encodeValue(intmax('int64'));
    assert_equal(minimumWire.intValue, '-9223372036854775808', 'encoded int64 minimum');
    assert_equal(maximumWire.intValue, '9223372036854775807', 'encoded int64 maximum');
    assert_error(@() opensysml.encodeValue(overflow), ...
        'opensysml:encode', 'encoded int64 overflow');
    assert_error(@() opensysml.internal.encodeNamedArguments( ...
        struct('overflow', overflow), conn), ...
        'opensysml:encode', 'named argument int64 overflow');

    verificationRecords = {struct('requirementId', 'R1'), ...
        struct('requirementId', 'R2')};
    summaryValues = opensysml.internal.decodeVerdicts( ...
        struct('kind', 'validation', 'holds', true), [], {}, verificationRecords, true);
    assertionValues = opensysml.internal.decodeVerdicts( ...
        struct('requirementId', 'R1'), [], {}, verificationRecords);
    assert_equal(numel(summaryValues{1}.verifications), 2, ...
        'summary retains all verification verdicts');
    assert_equal(numel(assertionValues{1}.verifications), 1, ...
        'assertion filters verification verdicts');
    validationInstances = {struct('id', int64(1)), struct('id', int64(2))};
    validation = opensysml.Validation({}, summaryValues{1}, validationInstances, ...
        {}, verificationRecords, false);
    assert_equal(iscell(validation.instances), true, 'validation instances are ordered cells');
    assert_equal(validation.instances{1}.id, int64(1), 'validation first instance order');
    emptyMap = containers.Map('KeyType', 'char', 'ValueType', 'any');
    encodedEmptyMap = opensysml.internal.encodeNamedArguments(emptyMap, conn);
    assert_equal(jsonencode(encodedEmptyMap), '{}', 'empty Map encodes as JSON object');
    assert_equal(jsonencode(opensysml.internal.encodeNamedArguments(struct(), conn)), ...
        '{}', 'empty struct encodes as JSON object');
    identifierArguments = containers.Map('KeyType', 'char', 'ValueType', 'any');
    identifierArguments('validName') = int64(1);
    encodedIdentifierArguments = ...
        opensysml.internal.encodeNamedArguments(identifierArguments, conn);
    assert_equal(isstruct(encodedIdentifierArguments), true, ...
        'identifier-keyed Map encodes as struct');
    assert_equal(~isempty(strfind(jsonencode(encodedIdentifierArguments), '"validName"')), ...
        true, 'identifier-keyed Map JSON object');
    namedArguments = containers.Map('KeyType', 'char', 'ValueType', 'any');
    namedArguments('this.isSolid') = int64(1);
    encodedArguments = opensysml.internal.encodeNamedArguments(namedArguments, conn);
    assert_equal(isa(encodedArguments, 'containers.Map'), true, ...
        'nonidentifier-keyed Map remains a Map');
    assert_equal(~isempty(strfind(jsonencode(encodedArguments), '"this.isSolid"')), ...
        true, 'encode unusual named-argument key');

    assert_error(@() opensysml.convert(conn, 'sysml'), ...
        'opensysml:argument', 'convert without a source');
    assert_error(@() opensysml.convert(conn, 'sysml', ...
        'content', 'package A;', 'modelHash', 'also-a-source'), ...
        'opensysml:argument', 'convert with multiple sources');
    assert_equal(opensysml.isV1(' MDZIP '), true, 'isV1 folds case and padding');
    assert_equal(opensysml.isV1('sysml'), false, 'isV1 of notation');
    assert_equal(opensysml.pathIsV1('dir/Model.XMI'), true, 'pathIsV1 by extension');
    assert_equal(opensysml.pathIsV1('Model.sysml'), false, 'pathIsV1 of notation');
    assert_error(@() opensysml.convert(conn, 'sysml', 'filePath', 'Model.mdzip'), ...
        'opensysml:argument', 'convert refuses a v1 file');
    assert_error(@() opensysml.convert(conn, 'sysml', 'content', '<xmi/>', 'fromFormat', 'XMI'), ...
        'opensysml:argument', 'convert refuses v1 content');
    last = opensysml.lastError();
    assert_equal(~isempty(strfind(last.message, 'migrated, not converted')), true, ...
        'convert refusal names migration');
    assert_equal(~isempty(strfind(last.message, 'opensysml.migrate')), true, ...
        'convert refusal points at migrate');
    assert_error(@() opensysml.migrate(conn, 'sysml'), ...
        'opensysml:argument', 'migrate without a source');
    assert_error(@() opensysml.migrate(conn, 'sysml', 'filePath', 'a.xmi', 'content', '<xmi/>'), ...
        'opensysml:argument', 'migrate with two sources');
    assert_error(@() opensysml.migrate(conn, 'sysml', 'filePath', 'Model.sysml', ...
        'fromFormat', 'sysml'), 'opensysml:argument', 'migrate refuses a v2 model');
    last = opensysml.lastError();
    assert_equal(~isempty(strfind(last.message, 'converted, not migrated')), true, ...
        'migrate refusal names conversion');
    assert_error(@() opensysml.migrate(conn, 'sysml', 'content', '<xmi/>'), ...
        'opensysml:argument', 'migrate content without fromFormat');
    assert_error(@() opensysml.migrate(conn, 'sysml', 'filePath', 'Model.mdzip', ...
        'layoutPath', 'a.xml', 'layoutContent', '<mtip/>'), ...
        'opensysml:argument', 'migrate with two layouts');
    assert_error(@() opensysml.migrate(conn, 'sysml', 'filePath', 'Model.mdzip', 'strict', 'yes'), ...
        'opensysml:argument', 'migrate strict must be logical');

    assert_equal(opensysml.internal.base64Decode(opensysml.internal.base64Encode(uint8([137 80 78 71]))), ...
        uint8([137 80 78 71]), 'base64 round trip');
    assert_equal(opensysml.internal.base64Decode('AA=='), uint8(0), 'base64 of one byte');
    warningState = warning;
    warning('off', 'opensysml:experimental');
    warningCleanup = onCleanup(@() warning(warningState));
    answer = struct('content', 'package Vehicle;', 'fromFormat', 'xmi', 'toFormat', 'sysml', ...
        'experimental', true, 'results', '{}', ...
        'report', struct('source', 'Vehicle.xmi', 'exporter', 'Cameo', 'summary', 's', ...
            'mapped', 2, 'approximated', 1, 'unmapped', 0, 'skipped', 1, 'text', 'report', ...
            'entries', {{struct('id', 'a', 'kind', 'Class', 'name', 'A', 'target', 'part def A', ...
                                'verdict', 'mapped', 'note', ''), ...
                         struct('id', 'd', 'kind', 'Diagram', 'name', 'D', 'target', '', ...
                                'verdict', 'skipped', 'note', 'diagram')}}), ...
        'files', {{struct('path', 'images/a.png', 'content', 'iVA=')}});
    migration = opensysml.Migration(answer, 'xmi', 'sysml', '');
    assert_equal(migration.content, 'package Vehicle;', 'migration content');
    assert_equal(migration.report.mapped, 2, 'migration mapped count');
    assert_equal(numel(migration.report.entries), 2, 'migration entries');
    skipped = migration.byVerdict('skipped');
    assert_equal(numel(skipped), 1, 'migration byVerdict');
    assert_equal(skipped{1}.id, 'd', 'migration skipped entry');
    assert_equal(migration.files('images/a.png'), uint8([137 80]), 'migration image bytes');
    assert_equal(migration.experimental, true, 'migration is experimental');
    assert_equal(~isempty(strfind(migration.experimentalNotice, 'experimental')), true, ...
        'migration notice filled in');
    directory = tempname;
    mkdir(directory);
    directoryCleanup = onCleanup(@() rmdir(directory, 's'));
    written = migration.write(fullfile(directory, 'Vehicle.sysml'));
    assert_equal(fileread(written), 'package Vehicle;', 'migration model written');
    assert_equal(exist(fullfile(directory, 'images', 'a.png'), 'file') == 2, true, ...
        'migration image written beside the model');
    escaping = {'../escaped.png', 'images/../../escaped.png', '/tmp/escaped.png', ...
        'images//x.png', 'images\x.png', 'Other.sysml'};
    for i = 1:numel(escaping)
        answer.files = {struct('path', escaping{i}, 'content', 'AA==')};
        bad = opensysml.Migration(answer, 'xmi', 'sysml', '');
        assert_error(@() bad.write(fullfile(directory, 'Other.sysml')), ...
            'opensysml:argument', ['escaping image ' escaping{i}]);
        assert_equal(exist(fullfile(directory, 'Other.sysml'), 'file'), 0, ...
            ['no model written for ' escaping{i}]);
    end
    source = fullfile(directory, 'Vehicle.xmi');
    fid = fopen(source, 'w'); fprintf(fid, '<xmi/>'); fclose(fid);
    answer.files = {};
    fromSource = opensysml.Migration(answer, 'xmi', 'sysml', source);
    assert_error(@() fromSource.write(source), 'opensysml:argument', 'overwriting the source');
    assert_equal(fileread(source), '<xmi/>', 'source left intact');
    delete(source);
    fromSource.write(source);
    assert_equal(fileread(source), 'package Vehicle;', 'a vacated source path protects nothing');
    if ~ispc
        fid = fopen(source, 'w'); fprintf(fid, '<xmi/>'); fclose(fid);
        alias = fullfile(directory, 'Alias.sysml');
        system(sprintf('ln -s "%s" "%s"', source, alias));
        assert_error(@() fromSource.write(alias), 'opensysml:argument', ...
            'overwriting the source through a link to it');
        assert_equal(fileread(source), '<xmi/>', 'source left intact behind its link');
        answer.files = {struct('path', 'Alias.sysml', 'content', 'AA==')};
        aliasing = opensysml.Migration(answer, 'xmi', 'sysml', source);
        assert_error(@() aliasing.write(fullfile(directory, 'Linked.sysml')), ...
            'opensysml:argument', 'an image aliasing the source through a link');
        assert_equal(fileread(source), '<xmi/>', 'source left intact behind an image link');
        outside = tempname;
        mkdir(outside);
        outsideCleanup = onCleanup(@() rmdir(outside, 's'));
        system(sprintf('ln -s "%s" "%s"', outside, fullfile(directory, 'linked')));
        mkdir(fullfile(directory, 'dangling'));
        system(sprintf('ln -s "%s" "%s"', fullfile(outside, 'missing'), ...
            fullfile(directory, 'dangling', 'dir')));
        system(sprintf('ln -s "%s" "%s"', fullfile(outside, 'file.png'), ...
            fullfile(directory, 'dangling', 'file.png')));
        mkdir(fullfile(directory, 'alias'));
        system(sprintf('ln -s "%s" "%s"', fullfile(directory, 'alias', 'real.png'), ...
            fullfile(directory, 'alias', 'alias.png')));
        linked = {'linked/escaped.png', 'dangling/dir/escaped.png', 'dangling/file.png', ...
            'alias/alias.png'};
        for i = 1:numel(linked)
            answer.files = {struct('path', linked{i}, 'content', 'AA==')};
            bad = opensysml.Migration(answer, 'xmi', 'sysml', '');
            assert_error(@() bad.write(fullfile(directory, 'Linked.sysml')), ...
                'opensysml:argument', ['linked image ' linked{i}]);
            assert_equal(exist(fullfile(directory, 'Linked.sysml'), 'file'), 0, ...
                ['no model written for ' linked{i}]);
        end
        assert_equal(numel(dir(outside)), 2, 'nothing written outside through a link');
        assert_equal(exist(fullfile(directory, 'alias', 'real.png'), 'file'), 0, ...
            'nothing written through a link at the image''s path');
        clear outsideCleanup;
    end
    clear directoryCleanup warningCleanup;
    assert_error(@() opensysml.runSweep(model, 'Demo::calc', struct('x', {{1}})), ...
        'opensysml:argument', 'invalid sweep range');

    assert_error(@() opensysml.executeAction(model, 'Demo::action', ...
        'schedule', 'explore'), 'opensysml:argument', 'executeAction exploration schedule');
    last = opensysml.lastError();
    assert_equal(~isempty(strfind(last.message, 'opensysml.exploreAction')), true, ...
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
    fprintf('surface offline ok\n');
end
