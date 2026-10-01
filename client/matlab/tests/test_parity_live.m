function test_parity_live()
%TEST_PARITY_LIVE Exercise parity APIs against the running service.

    if isempty(getenv('OPENSYSML_SERVICE'))
        fprintf('parity live: skipped (OPENSYSML_SERVICE unset)\n');
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
    assert_equal(isa(simple.find('SimplePart'), 'opensysml.Symbol'), ...
        true, 'Model.find');
    assert_equal(isa(simple.lookup('SimplePart'), 'opensysml.Symbol'), ...
        true, 'Model.lookup');
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
    textRows = opensysml.query(queryModel, ...
        'oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name');
    structuredRows = queryModel.query('scope', {'Demo::vehicle'}, 'select', {'name'});
    assert_equal(isstruct(textRows) && ~isempty(textRows.elements), true, 'OSLC query');
    assert_equal(iscell(structuredRows), true, 'structured Model.query');

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
    markdown = opensysml.renderDocument(documentModel, 'Observatory::MassReport');
    html = documentModel.renderDocument('Observatory::MassReport', 'form', 'html');
    assert_equal(~isempty(strfind(markdown, 'Telescope Mass Report')), true, ...
        'Markdown document rendering');
    assert_equal(~isempty(strfind(html, '<!DOCTYPE html>')), true, ...
        'HTML Model.renderDocument');

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
    expect_identifier(@() opensysml.parseFile(conn, '/no-such-parity-model.sysml'), ...
        'opensysml:connect:modelFileNotFound', 'modelFileNotFound');
    expect_identifier(@() opensysml.calc(verification, 'Demo::Vehicle::massPositive'), ...
        'opensysml:diagnostics:wrongKind', 'wrongKind');
    expect_identifier(@() opensysml.convert(conn, 'ttl', 'content', ...
        'package Broken { part def ', 'fromFormat', 'sysml'), ...
        'opensysml:diagnostics:conversion', 'conversion');
    expect_identifier(@() opensysml.parseSource(conn, 'package Broken {', ...
        'name', 'broken.sysml', 'raiseForErrors', true), ...
        'opensysml:diagnostics:model', 'model');
    fprintf('parity live ok\n');
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
