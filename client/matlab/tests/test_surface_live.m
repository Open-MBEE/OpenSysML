function test_surface_live()
%TEST_SURFACE_LIVE Exercise client APIs against the running service.

    if isempty(getenv('OPENSYSML_SERVICE'))
        fprintf('surface live: skipped (OPENSYSML_SERVICE unset)\n');
        return;
    end
    conn = opensysml.connect();
    repo = fileparts(fileparts(fileparts(fileparts(mfilename('fullpath')))));
    fixtures = fullfile(repo, 'conformance', 'fixtures');

    [version, capabilities] = conn.serverInfo();
    assert_equal(ischar(version) && ~isempty(version), true, 'server version');
    assert_equal(iscell(capabilities), true, 'server capabilities');
    engines = opensysml.listEngines(conn);
    assert_equal(iscell(engines), true, 'engine list');

    simple = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'simple_part.sysml')), ...
        'name', 'simple_part.sysml');
    assert_equal(simple.ok(), true, 'model ok');
    parsedFile = opensysml.parseFile(conn, 'conformance/fixtures/simple_part.sysml', ...
        'language', 'sysml');
    assert_equal(parsedFile.ok(), true, 'parseFile language option');
    assert_equal(isempty(simple.errors()), true, 'model errors');
    assert_equal(isempty(simple.raiseForErrors()), false, 'raiseForErrors result');
    symbol = simple.symbol('Test::SimplePart');
    assert_equal(isa(symbol, 'opensysml.Symbol'), true, 'Model.symbol');
    assert_equal(iscell(symbol.children()), true, 'Symbol.children');
    symbolAttributes = symbol.attributes();
    assert_equal(iscell(symbolAttributes), true, 'Symbol.attributes');
    assert_equal(iscell(symbol.parts()), true, 'Symbol.parts');
    assert_equal(iscell(symbol.attributeFacts()), true, 'Symbol.attributeFacts');
    facts = symbol.facts();
    assert_equal(facts.id, 'Test::SimplePart', 'Symbol.facts');
    if ~isempty(symbolAttributes)
        attribute = symbolAttributes{1};
        assert_equal(isa(symbol.getAttr(attribute.name), 'opensysml.Symbol'), ...
            true, 'Symbol.getAttr');
    end
    assert_equal(isa(simple.get('Test::SimplePart'), 'opensysml.Symbol'), ...
        true, 'Model.get');
    assert_equal(isempty(simple.get('X::NoSuchSymbol')), true, 'missing Model.get symbol');
    assert_equal(isa(simple.find('SimplePart'), 'opensysml.Symbol'), ...
        true, 'Model.find');
    assert_equal(~isempty(simple.walk()), true, 'Model.walk');
    assert_equal(simple.evaluate('2 + 2'), int64(4), 'Model.evaluate');
    instance = simple.instantiate('Test::SimplePart');
    assert_equal(isa(instance.id, 'int64'), true, 'Model.instantiate');
    strictSimple = opensysml.parseSource(conn, ...
        fileread(fullfile(fixtures, 'simple_part.sysml')), ...
        'name', 'simple_strict.sysml', 'strictConformance', true);
    assert_equal(strictSimple.ok(), true, 'strict parse');

    directConversion = opensysml.convert(conn, 'sysml', 'content', ...
        'package Inline { part def P; }', 'fromFormat', 'sysml');
    assert_equal(ischar(directConversion.content), true, 'convert');
    sysmlConversion = simple.toSysml();
    modelConversion = simple.convert('sysml');
    assert_equal(ischar(sysmlConversion.content), true, 'Model.toSysml');
    assert_equal(ischar(modelConversion.content), true, 'Model.convert');
    warningState = warning;
    warning('off', 'opensysml:experimental');
    warningCleanup = onCleanup(@() warning(warningState));
    vehicle = fullfile(fixtures, 'vehicle.xmi');
    assert_error(@() opensysml.convert(conn, 'sysml', 'filePath', vehicle), ...
        'opensysml:argument', 'convert refuses a v1 model');
    migrated = opensysml.migrate(conn, 'sysml', 'filePath', vehicle, 'report', true);
    assert_equal(~isempty(strfind(migrated.content, 'part def Vehicle')), true, 'migrate');
    assert_equal(migrated.fromFormat, 'xmi', 'migrate from format');
    assert_equal([migrated.report.mapped, migrated.report.approximated, ...
        migrated.report.unmapped, migrated.report.skipped], [78 12 3 2], 'migration counts');
    assert_equal(numel(migrated.report.entries), 95, 'migration entries');
    assert_equal(numel(migrated.byVerdict('unmapped')), 3, 'migration unmapped entries');
    assert_equal(migrated.sourcePath, opensysml.internal.absolutePath(vehicle), 'migration source path');
    fid = fopen(vehicle, 'rb'); vehicleBytes = fread(fid, Inf, 'uint8=>uint8')'; fclose(fid);
    inlineMigration = opensysml.migrate(conn, 'ttl', 'content', vehicleBytes, 'fromFormat', ' XMI ');
    assert_equal(inlineMigration.toFormat, 'ttl', 'inline migration to Turtle');
    assert_equal(inlineMigration.report.mapped, 78, 'inline migration counts');
    assert_equal(isempty(inlineMigration.report.entries), true, 'inline migration summary only');
    assert_equal(isempty(inlineMigration.sourcePath), true, 'inline migration has no source path');
    migratedPath = [tempname '.sysml'];
    migrated.write(migratedPath);
    migratedCleanup = onCleanup(@() delete(migratedPath));
    assert_equal(fileread(migratedPath), migrated.content, 'migration written');
    simple.toTurtle();
    simple.toApiJson();
    clear warningCleanup;
    savePath = [tempname(fileparts(mfilename('fullpath'))) '.sysml'];
    cleanup = onCleanup(@() deleteIfPresent(savePath));
    saved = simple.save(savePath, 'format', 'sysml');
    assert_equal(exist(savePath, 'file'), 2, 'Model.save output');
    assert_equal(ischar(saved.content), true, 'Model.save result');
    clear cleanup;

    parsed = opensysml.parseSources(conn, ...
        {{'first.sysml', 'package First { part def A; }'}, ...
         {'second.sysml', 'package Second { part def B; }'}}, ...
        'language', 'sysml');
    assert_equal(numel(parsed.roots), 2, 'parseSources roots');

    queryModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'query.sysml')), ...
        'name', 'query.sysml');
    reservedRows = queryModel.query('select', {'@id', '@type'});
    reservedProperties = reservedRows{1}.properties;
    assert_equal(isa(reservedProperties, 'containers.Map'), true, ...
        'query reserved properties map');
    assert_equal(isKey(reservedProperties, '@id'), true, 'query preserves @id');
    assert_equal(isKey(reservedProperties, '@type'), true, 'query preserves @type');
    textRows = opensysml.query(queryModel, ...
        'oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name');
    structuredRows = queryModel.query('scope', {'Demo::vehicle'}, 'select', {'name'});
    filter = struct('property', 'name', 'operator', '=', 'value', 'vehicle');
    optionRows = queryModel.query('where', filter);
    builtRows = queryModel.query(opensysml.buildQuery([], 'where', filter));
    assert_equal(isstruct(textRows) && ~isempty(textRows.elements), true, 'OSLC query');
    assert_equal(iscell(structuredRows), true, 'structured Model.query');
    assert_equal(~isempty(optionRows), true, 'structured query where rows');
    optionIds = cellfun(@(item) item.id, optionRows, 'UniformOutput', false);
    builtIds = cellfun(@(item) item.id, builtRows, 'UniformOutput', false);
    assert_equal(isequal(optionIds, builtIds), true, 'built structured Model.query');

    documentPath = fullfile(repo, 'internal', 'doc', 'docrender', ...
        'testdata', 'telescope_report.sysml');
    documentModel = opensysml.parseSource(conn, fileread(documentPath), ...
        'name', 'telescope_report.sysml');
    documentQuery = opensysml.runDocumentQuery(documentModel, ...
        'Observatory::SubsystemTable', 'bindings', ...
        struct('root', struct('type', 'element', 'id', 'Observatory::telescope')));
    methodQuery = documentModel.runDocumentQuery('Observatory::SubsystemTable', ...
        'bindings', struct('root', struct('type', 'element', 'id', 'Observatory::telescope')));
    assert_equal(iscell(documentQuery.columns), true, 'document query columns');
    assert_equal(numel(documentQuery.rows) > 0, true, 'document query rows');
    assert_equal(numel(methodQuery.rows), numel(documentQuery.rows), 'Model.runDocumentQuery');
    statePath = fullfile(repo, 'internal', 'doc', 'docrender', ...
        'testdata', 'state_report.sysml');
    stateModel = opensysml.parseSource(conn, fileread(statePath), ...
        'name', 'state_report.sysml');
    stateModel.instantiate('Lamps::lamp1');
    stateResult = stateModel.runDocumentQuery('Lamps::CurrentStates');
    stateRow = stateResult.rows{1};
    assert_equal(stateRow.state.type, 'state', 'document state row');
    assert_equal(stateRow.object.type, 'object', 'document state object');
    assert_equal(stateRow.element.id, 'Lamps::lamp1', 'document state row element');
    markdown = opensysml.renderDocument(documentModel, 'Observatory::MassReport');
    html = documentModel.renderDocument('Observatory::MassReport', 'form', 'html');
    assert_equal(~isempty(strfind(markdown, 'Telescope Mass Report')), true, ...
        'Markdown document rendering');
    assert_equal(~isempty(strfind(html, '<!DOCTYPE html>')), true, ...
        'HTML Model.renderDocument');
    viewModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'views.sysml')), ...
        'name', 'views.sysml');
    renderedView = opensysml.renderView(viewModel, 'RenderViewDemo::connections');
    renderedViewMethod = viewModel.renderView('RenderViewDemo::connections');
    assert_equal(strcmp(renderedView.kind, 'interconnection'), true, 'RenderView kind');
    assert_equal(numel(renderedView.edges), 1, 'RenderView edges');
    assert_equal(~isempty(renderedView.edges(1).fromPort), true, 'RenderView from port');
    assert_equal(~isempty(renderedView.edges(1).toPort), true, 'RenderView to port');
    assert_equal(all(arrayfun(@(node) ~isempty(node.origin), renderedView.nodes)), ...
        true, 'RenderView origins');
    assert_equal(numel(renderedViewMethod.edges), numel(renderedView.edges), ...
        'Model.renderView');
    fullView = opensysml.renderView(viewModel, 'RenderViewDemo::connections', 'ports', 'full');
    allPortNames = {};
    for n = 1:numel(fullView.nodes)
        allPortNames = [allPortNames, {fullView.nodes(n).ports.name}];
    end
    assert_equal(any(strcmp(allPortNames, 'spare')), true, 'RenderView full ports');

    behavior = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'behavior.sysml')), ...
        'name', 'behavior.sysml');
    action = opensysml.executeAction(behavior, 'Test::addFive', ...
        'inputs', struct('result', int64(10)));
    actionMethod = behavior.executeAction('Test::addFive', ...
        'inputs', struct('result', int64(10)));
    assert_equal(action.outputs.result, int64(15), 'executeAction');
    assert_equal(actionMethod.outputs.result, int64(15), 'Model.executeAction');
    assert_equal(isa(action.performerAttributes, 'containers.Map'), true, ...
        'action performer attributes');
    assert_equal(isnumeric(action.finalTime), true, 'action final time');
    performerSource = strjoin({ ...
        'package Wire {', ...
        '  private import ScalarValues::*;', ...
        '  item def Ping;', ...
        '  port def Link { in item ping : Ping; }', ...
        '  part def Craft {', ...
        '    attribute pinged : Boolean = false;', ...
        '    action look { out seen : Boolean; first start;', ...
        '      then action read assign seen := pinged; then done; }', ...
        '  }', ...
        '  part def Pair { part craft : Craft; }', ...
        '  part pair : Pair;', ...
        '}'}, sprintf('\n'));
    performerModel = opensysml.parseSource(conn, performerSource, ...
        'name', 'performer.sysml');
    performerResult = performerModel.executeAction('Wire::Craft::look', ...
        'performer', 'Wire::pair.craft');
    performerKeys = performerResult.performerAttributes.keys;
    assert_equal(any(cellfun(@(key) strncmp(key, 'this.', 5), performerKeys)), ...
        true, 'performer this-prefixed keys');
    actionExploration = opensysml.exploreAction(behavior, 'Test::race');
    actionExplorationMethod = behavior.exploreAction('Test::race');
    assert_equal(isa(actionExploration, 'opensysml.Exploration'), true, 'exploreAction');
    assert_equal(isa(actionExplorationMethod, 'opensysml.Exploration'), true, ...
        'Model.exploreAction');
    state = opensysml.executeState(behavior, 'Test::Machine');
    stateMethod = behavior.executeState('Test::Machine');
    assert_equal(iscell(state.statesVisited), true, 'executeState');
    assert_equal(iscell(stateMethod.statesVisited), true, 'Model.executeState');
    assert_equal(isnumeric(state.finalTime), true, 'state final time');
    stateExploration = opensysml.exploreState(behavior, 'Test::Machine');
    stateExplorationMethod = behavior.exploreState('Test::Machine');
    assert_equal(isa(stateExploration, 'opensysml.Exploration'), true, 'exploreState');
    assert_equal(isa(stateExplorationMethod, 'opensysml.Exploration'), true, ...
        'Model.exploreState');

    verification = opensysml.parseSource(conn, ...
        fileread(fullfile(fixtures, 'verification.sysml')), 'name', 'verification.sysml');
    cases = opensysml.parseSource(conn, ...
        fileread(fullfile(fixtures, 'verification_cases.sysml')), ...
        'name', 'verification_cases.sysml');
    constraintVerdict = opensysml.verifyConstraint(verification, ...
        'Demo::Vehicle::massPositive');
    constraintMethod = verification.verifyConstraint('Demo::Vehicle::massPositive');
    assert_equal(isa(constraintVerdict, 'opensysml.Verdict'), true, 'verifyConstraint');
    assert_equal(isa(constraintMethod, 'opensysml.Verdict'), true, ...
        'Model.verifyConstraint');
    requirementVerdict = opensysml.verifyRequirement(cases, 'Demo::Zeroed', ...
        'subject', 'Demo::good');
    requirementMethod = cases.verifyRequirement('Demo::Zeroed', 'subject', 'Demo::good');
    assert_equal(isa(requirementVerdict, 'opensysml.Verdict'), true, 'verifyRequirement');
    assert_equal(isa(requirementMethod, 'opensysml.Verdict'), true, ...
        'Model.verifyRequirement');
    satisfaction = opensysml.verifySatisfaction(cases, 'symbol', 'Demo::checks');
    satisfactionMethod = cases.verifySatisfaction('symbol', 'Demo::checks');
    assert_equal(iscell(satisfaction), true, 'verifySatisfaction');
    assert_equal(iscell(satisfactionMethod), true, 'Model.verifySatisfaction');
    cases.satisfied('Demo::checks');
    cases.validateInstance('Demo::good');
    validation = opensysml.validateInstance(cases, 'Demo::good');
    assert_equal(isa(validation, 'opensysml.Validation'), true, 'validateInstance');
    assert_equal(iscell(validation.instances), true, 'validation ordered instances');
    assert_equal(~isempty(validation.instances), true, 'validation instance list');
    assert_equal(validation.instances{1}.id, validation.summary.instanceId, ...
        'validated object is first in instance list');
    assert_equal(numel(validation.summary.verifications), ...
        numel(validation.verifications), 'summary receives all verification verdicts');
    assert_equal(isa(verification.calc('Demo::add', 'arguments', {int64(2), int64(3)}), ...
        'opensysml.CalcResult'), true, 'Model.calc');
    calculation = opensysml.calc(verification, 'Demo::add', ...
        'arguments', {int64(2), int64(3)});
    assert_equal(isa(calculation, 'opensysml.CalcResult'), true, 'calc');

    analysis = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'analysis.sysml')), ...
        'name', 'analysis.sysml');
    result = opensysml.runAnalysis(analysis, 'An::plain');
    resultMethod = analysis.runAnalysis('An::plain');
    assert_equal(result.outputs.x, 3, 'runAnalysis');
    assert_equal(resultMethod.outputs.x, 3, 'Model.runAnalysis');
    try
        opensysml.runAnalysis(analysis, 'An::CostAnalysis');
        error('assert:error', 'runAnalysis without a subject returned normally');
    catch e
        if strcmp(e.identifier, 'assert:error'), rethrow(e); end
        assert_equal(e.identifier, 'opensysml:diagnostics:analysisRun', ...
            'partial analysis error identifier');
        partial = opensysml.lastError();
        assert_equal(isfield(partial.details, 'result'), true, ...
            'partial analysis result detail');
        assert_equal(isa(partial.details.result, 'opensysml.AnalysisResult'), true, ...
            'partial analysis result class');
    end
    analysisExploration = opensysml.exploreAnalysis(analysis, 'An::plain');
    analysisExplorationMethod = analysis.exploreAnalysis('An::plain');
    assert_equal(isa(analysisExploration, 'opensysml.Exploration'), true, ...
        'exploreAnalysis');
    assert_equal(isa(analysisExplorationMethod, 'opensysml.Exploration'), true, ...
        'Model.exploreAnalysis');

    sweepModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'sweep.sysml')), ...
        'name', 'sweep.sysml');
    ranges = struct('a', [1 2], 'b', [3 4]);
    table = opensysml.runSweep(sweepModel, 'Sw::Sum', ranges);
    mapRanges = containers.Map('KeyType', 'char', 'ValueType', 'any');
    mapRanges('a') = struct('from', 1, 'to', 2);
    mapRanges('b') = struct('from', 3, 'to', 4);
    tableMethod = sweepModel.runSweep('Sw::Sum', mapRanges);
    assert_equal(isa(table, 'opensysml.SweepTable'), true, 'runSweep');
    assert_equal(numel(table.rows), 4, 'sweep cartesian product');
    assert_equal(numel(tableMethod.rows), 4, 'Model.runSweep');

    expect_identifier(@() opensysml.evaluate( ...
        opensysml.Model(conn, 'missing-model-hash', {}), '2 + 2'), ...
        'opensysml:connect:modelNotFound', 'modelNotFound');
    expect_identifier(@() opensysml.runDocumentQuery(documentModel, ...
        'Observatory::NoSuchQuery'), ...
        'opensysml:connect:symbolNotFound', 'symbolNotFound');
    expect_identifier(@() opensysml.parseFile(conn, '/no-such-matlab-model.sysml'), ...
        'opensysml:connect:modelFileNotFound', 'modelFileNotFound');
    expect_identifier(@() opensysml.calc(verification, 'Demo::Vehicle::massPositive'), ...
        'opensysml:diagnostics:wrongKind', 'wrongKind');
    expect_identifier(@() opensysml.convert(conn, 'ttl', 'content', ...
        'package Broken { part def ', 'fromFormat', 'sysml'), ...
        'opensysml:diagnostics:conversion', 'conversion');
    expect_identifier(@() opensysml.parseSource(conn, 'package Broken {', ...
        'name', 'broken.sysml', 'raiseForErrors', true), ...
        'opensysml:diagnostics:model', 'model');

    measurementModel = opensysml.parseFile(conn, ...
        'conformance/fixtures/measurement_ref.sysml');
    walkedMeasurement = measurementModel.walk(4);
    assert_equal(iscell(walkedMeasurement), true, 'walk skips missing child symbol');

    closedConnection = opensysml.external(getenv('OPENSYSML_SERVICE'));
    closedConnection.close();
    closedConnection.close();
    try
        opensysml.listEngines(closedConnection);
        error('assert:error', 'call after close returned normally');
    catch e
        if strcmp(e.identifier, 'assert:error'), rethrow(e); end
        assert_equal(e.identifier, 'opensysml:transport', 'closed connection identifier');
        assert_equal(~isempty(strfind(e.message, 'is closed')), true, ...
            'closed connection message');
    end

    deadConnection = opensysml.external('127.0.0.1:1');
    expect_transport_without_marker(@() opensysml.call( ...
        deadConnection, 'GetServerInfo', struct()));

    timeoutConnection = opensysml.external(getenv('OPENSYSML_SERVICE'));
    timeoutConnection.serverInfo();
    timeoutConnection.timeout = 0.001;
    timeoutCleanup = onCleanup(@() timeoutConnection.close());
    timeoutLines = cell(1, 12000);
    for i = 1:numel(timeoutLines)
        timeoutLines{i} = sprintf('part def TimeoutPart%d;', i);
    end
    largeSource = strjoin(timeoutLines, sprintf('\n'));
    expect_identifier(@() opensysml.parseSource(timeoutConnection, largeSource, ...
        'name', 'timeout.sysml'), ...
        'opensysml:connect:serviceTimeout', 'subsecond parse timeout');
    clear timeoutCleanup;
    fprintf('surface live ok\n');
end

function deleteIfPresent(path)
    if exist(path, 'file'), delete(path); end
end

function expect_identifier(fn, prefix, label)
    try
        fn();
    catch e
        if strncmp(e.identifier, prefix, numel(prefix)), return; end
        error('assert:error', '%s threw %s, want %s*', label, e.identifier, prefix);
    end
    error('assert:error', '%s returned normally, want %s*', label, prefix);
end

function expect_transport_without_marker(fn)
    try
        fn();
    catch e
        if strcmp(e.identifier, 'assert:error'), rethrow(e); end
        assert_equal(e.identifier, 'opensysml:transport', 'closed-port transport identifier');
        assert_equal(isempty(strfind(e.message, '__OPENSYSML_STATUS__')), true, ...
            'closed-port error hides status marker');
        return;
    end
    error('assert:error', 'closed-port call returned normally');
end
