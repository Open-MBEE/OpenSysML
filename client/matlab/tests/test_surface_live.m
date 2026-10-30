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
    probed = opensysml.listEngines(conn, 'probe', true);
    assert_equal(numel(probed), numel(engines), 'probed engine list');

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
    assert_equal(isempty(modelConversion.libraryReferences), true, 'notation names no library references');
    warningState = warning;
    warning('off', 'opensysml:experimental');
    warningCleanup = onCleanup(@() warning(warningState));
    librarySource = 'package P { private import SI::*; attribute time = 20 [s]; }';
    plain = opensysml.convert(conn, 'api-json', 'content', librarySource, 'fromFormat', 'sysml');
    referenceNames = cellfun(@(ref) ref.qualifiedName, plain.libraryReferences, 'UniformOutput', false);
    second = plain.libraryReferences{find(strcmp(referenceNames, 'SI::second'), 1)};
    assert_equal(second.id, '0c539f7c-20a1-520d-bae0-f3f855517aea', 'library reference id');
    assert_equal(second.type, 'AttributeUsage', 'library reference type');
    assert_equal(isempty(strfind(plain.content, '"isLibraryElement":true')) && ...
        isempty(strfind(plain.content, '"isLibraryElement": true')), true, 'no stubs by default');
    legacy = opensysml.convert(conn, 'api-json', 'content', librarySource, 'fromFormat', 'sysml', ...
        'includeLibraryStubs', true);
    assert_equal(isempty(strfind(legacy.content, second.id)), false, 'stub written for the reference');
    assert_equal(isempty(strfind(legacy.content, '"isLibraryElement":true')) && ...
        isempty(strfind(legacy.content, '"isLibraryElement": true')), false, 'stubs on request');
    assert_equal(numel(legacy.libraryReferences), numel(plain.libraryReferences), 'stubs keep the references');
    qualifiedIds = opensysml.convert(conn, 'api-json', 'content', 'package P { part def A; part a : A; }', ...
        'fromFormat', 'sysml');
    uuidIds = opensysml.convert(conn, 'api-json', 'content', 'package P { part def A; part a : A; }', ...
        'fromFormat', 'sysml', 'idForm', 'uuid');
    assert_equal(isempty(strfind(qualifiedIds.content, '"P__a"')), false, 'qualified ids by default');
    assert_equal(isempty(strfind(uuidIds.content, '"P__a"')), true, 'uuid ids on request');
    idModel = opensysml.parseSource(conn, 'package P { part def A; part a : A; }', 'name', 'ids.sysml');
    assert_equal(isempty(strfind(idModel.convert('api-json', 'idForm', 'uuid').content, '"P__a"')), ...
        true, 'Model.convert forwards idForm');
    libraryModel = opensysml.parseSource(conn, librarySource, 'name', 'library.sysml');
    modelLegacy = libraryModel.convert('api-json', 'includeLibraryStubs', true);
    assert_equal(isempty(strfind(modelLegacy.content, second.id)), false, 'Model.convert forwards library stubs');
    assert_equal(numel(modelLegacy.libraryReferences), numel(plain.libraryReferences), 'Model.convert keeps references');
    expect_identifier(@() opensysml.convert(conn, 'api-json', 'content', 'package P;', ...
        'fromFormat', 'sysml', 'idForm', 'short'), 'opensysml:connect:invalidRequest', 'unknown id form');
    compactSource = 'package Compact { part def A; part a : A; }';
    compactJson = opensysml.convert(conn, 'api-json', 'content', compactSource, 'fromFormat', 'sysml', ...
        'compact', true);
    assert_equal(isempty(strfind(compactJson.content, 'api-json-compact/1')), false, 'compact document');
    assert_equal(isempty(strfind(compactJson.content, '"owner"')), false, 'compact keeps derived properties');
    trimmed = opensysml.convert(conn, 'api-json', 'content', compactSource, 'fromFormat', 'sysml', ...
        'compact', true, 'omitDerived', true);
    assert_equal(isempty(strfind(trimmed.content, '"owner"')), true, 'omitDerived drops owner');
    kept = opensysml.convert(conn, 'api-json', 'content', compactSource, 'fromFormat', 'sysml', ...
        'compact', true, 'omitDerived', true, 'keepDerived', {'owner'});
    assert_equal(isempty(strfind(kept.content, '"owner"')), false, 'keepDerived keeps owner');
    expect_identifier(@() opensysml.convert(conn, 'api-json', 'content', compactSource, ...
        'fromFormat', 'sysml', 'omitDerived', true), 'opensysml:connect:invalidRequest', ...
        'omitDerived needs compact');
    compactModel = opensysml.parseSource(conn, compactSource, 'name', 'compact.sysml');
    assert_equal(isempty(strfind(compactModel.convert('api-json', 'compact', true, 'omitDerived', true).content, ...
        '"owner"')), true, 'Model.convert forwards the compact options');
    brokenSource = 'package Broken { part def A; part a : Missing; }';
    plainBroken = opensysml.convert(conn, 'api-json', 'content', brokenSource, 'fromFormat', 'sysml');
    checkedBroken = opensysml.convert(conn, 'api-json', 'content', brokenSource, 'fromFormat', 'sysml', ...
        'includeCheckDiagnostics', true);
    assert_equal(isempty(plainBroken.diagnostics), true, 'no check diagnostics by default');
    assert_equal(isempty(checkedBroken.diagnostics), false, 'check diagnostics on request');
    severities = cellfun(@(d) d.severity, checkedBroken.diagnostics, 'UniformOutput', false);
    assert_equal(any(strcmp(severities, 'error')), true, 'check diagnostics keep their severity');
    checkedClean = opensysml.convert(conn, 'api-json', 'content', compactSource, 'fromFormat', 'sysml', ...
        'includeCheckDiagnostics', true);
    assert_equal(isempty(checkedClean.diagnostics), true, 'a clean model checks clean');
    affectedLib = {'octave-affected-lib.sysml', 'package AffectedLib { part def Engine; }'};
    affectedTop = 'package AffectedTop { private import AffectedLib::*; part engine : Engine; }';
    opensysml.parseSources(conn, {affectedLib, {'octave-affected-top.sysml', [affectedTop ' // seed']}});
    affectedBase = opensysml.parseSources(conn, {affectedLib, {'octave-affected-top.sysml', affectedTop}});
    assert_equal(isempty(affectedBase.affected), true, 'no affected documents without a base');
    unchanged = opensysml.parseSources(conn, {affectedLib, {'octave-affected-top.sysml', affectedTop}}, ...
        'baseModelHash', affectedBase.hash);
    assert_equal(isempty(unchanged.affected), true, 'an unchanged re-parse affects nothing');
    edited = opensysml.parseSources(conn, {affectedLib, {'octave-affected-top.sysml', ...
        strrep(affectedTop, 'part engine', 'part motor')}}, 'baseModelHash', affectedBase.hash);
    assert_equal(edited.affected, {'octave-affected-top.sysml'}, 'an edit names its document');
    vehicle = fullfile(fixtures, 'vehicle.xmi');
    assert_error(@() opensysml.convert(conn, 'sysml', 'filePath', vehicle), ...
        'opensysml:argument', 'convert refuses a v1 model');
    migrated = opensysml.migrate(conn, 'sysml', 'filePath', vehicle, 'report', true);
    assert_equal(~isempty(strfind(migrated.content, 'part def Vehicle')), true, 'migrate');
    assert_equal(migrated.fromFormat, 'xmi', 'migrate from format');
    assert_equal([migrated.report.mapped, migrated.report.approximated, ...
        migrated.report.unmapped, migrated.report.skipped], [77 15 3 2], 'migration counts');
    assert_equal(numel(migrated.report.entries), 97, 'migration entries');
    assert_equal(numel(migrated.byVerdict('unmapped')), 3, 'migration unmapped entries');
    assert_equal(migrated.sourcePath, opensysml.internal.absolutePath(vehicle), 'migration source path');
    fid = fopen(vehicle, 'rb'); vehicleBytes = fread(fid, Inf, 'uint8=>uint8')'; fclose(fid);
    inlineMigration = opensysml.migrate(conn, 'ttl', 'content', vehicleBytes, 'fromFormat', ' XMI ');
    assert_equal(inlineMigration.toFormat, 'ttl', 'inline migration to Turtle');
    assert_equal(inlineMigration.report.mapped, 77, 'inline migration counts');
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
    assert_equal(any(strcmp(renderedView.shown, 'port-symbols')), true, 'RenderView shown');
    shownView = opensysml.renderView(viewModel, 'RenderViewDemo::connections', 'show', {'edge-labels'});
    assert_equal(any(strcmp(shownView.shown, 'edge-labels')), true, 'RenderView show');
    deepSource = strjoin({ ...
        'package Deep {', ...
        '  private import Views::*;', ...
        '  part vehicle {', ...
        '    part wheel { part hub { part bearing; } }', ...
        '    part engine { part piston; }', ...
        '  }', ...
        '  view structure { expose vehicle; render asTreeDiagram; }', ...
        '}'}, sprintf('\n'));
    deepModel = opensysml.parseSource(conn, deepSource, 'name', 'deep.sysml');
    wholeView = opensysml.renderView(deepModel, 'Deep::structure');
    assert_equal(numel(wholeView.nodes), 6, 'RenderView whole depth');
    boundedView = deepModel.renderView('Deep::structure', 'depth', 1);
    assert_equal(numel(boundedView.nodes), 3, 'RenderView depth 1');
    assert_equal(any(~cellfun(@isempty, strfind(boundedView.notices, 'nested deeper than 1 level'))), ...
        true, 'RenderView depth notice');

    lintSource = strjoin({ ...
        'package test {', ...
        '  private import ScalarValues::*;', ...
        '  attribute x : Real = 0.1;', ...
        '  state def M { entry; then a; state a; state b; transition first a when Pnig then b; }', ...
        '}'}, sprintf('\n'));
    linted = opensysml.parseSource(conn, lintSource, 'name', 'lints.sysml');
    assert_equal(any(strcmp(diagnosticCodes(linted.diagnostics), 'undeclared-signal')), true, ...
        'parse diagnostics carry the default lint');
    refreshed = opensysml.refreshDiagnostics(linted);
    assert_equal(any(strcmp(diagnosticCodes(refreshed), 'undeclared-signal')), true, ...
        'refreshDiagnostics default selection');
    disabled = linted.refreshDiagnostics('disableLint', {'undeclared-signal'});
    assert_equal(any(strcmp(diagnosticCodes(disabled), 'undeclared-signal')), false, ...
        'refreshDiagnostics disableLint');
    assert_equal(any(strcmp(diagnosticCodes(linted.diagnostics), 'rounded-real-literal')), false, ...
        'opt-in lint is off by default');
    enabled = opensysml.refreshDiagnostics(linted, 'enableLint', {'rounded-real-literal'});
    assert_equal(any(strcmp(diagnosticCodes(enabled), 'rounded-real-literal')), true, ...
        'refreshDiagnostics enableLint');
    assert_error(@() opensysml.refreshDiagnostics(linted, 'enableLint', {'no-such-lint'}), ...
        'opensysml:', 'refreshDiagnostics unknown lint');

    behaviorModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'behavior.sysml')), ...
        'name', 'behavior.sysml');
    graphs = opensysml.exportGraphs(behaviorModel, 'Test::race');
    graphsMethod = behaviorModel.exportGraphs('Test::race');
    assert_equal(graphs.version, 1, 'ExportGraphs version');
    assert_equal(graphs.subject, 'Test::race', 'ExportGraphs subject');
    assert_equal(strncmp(graphs.content, '{"version":1,"subject":"Test::race"', 35), true, 'ExportGraphs content');
    assert_equal(strcmp(graphs.content, graphsMethod.content), true, 'ExportGraphs method');

    tankModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'tank.sysml')), ...
        'name', 'tank.sysml');
    checked = opensysml.checkAction(tankModel, 'Plant::Tank::fill', 'performer', 'Plant::tank', ...
        'properties', {'Plant::Tank::low'}, 'diverge', {'this.level'}, 'witness', true);
    assert_equal(checked.verdict, 'fails', 'CheckBehavior verdict');
    assert_equal(checked.search.states, 11, 'CheckBehavior states');
    assert_equal(numel(checked.violations), 1, 'CheckBehavior violations');
    assert_equal(checked.violations{1}.property, 'Plant::Tank::low', 'CheckBehavior property');
    assert_equal(checked.violations{1}.depth, 3, 'CheckBehavior depth');
    assert_equal(isempty(checked.violations{1}.witness.fileName), false, 'CheckBehavior witness file');
    assert_equal(numel(checked.divergences), 1, 'CheckBehavior divergences');
    assert_equal(checked.divergences{1}.values{2}.value, '2', 'CheckBehavior divergent value');
    held = tankModel.checkAction('Plant::Tank::fill', 'performer', 'Plant::tank', ...
        'properties', {'Plant::Tank::capped'}, 'diverge', {'this.capacity'});
    assert_equal(held.verdict, 'holds', 'CheckBehavior holds');
    assert_equal(all(arrayfun(@(node) ~isempty(node.origin), renderedView.nodes)), ...
        true, 'RenderView origins');
    assert_equal(numel(renderedViewMethod.edges), numel(renderedView.edges), ...
        'Model.renderView');
    boxes = opensysml.renderView(viewModel, '#tree:RenderViewDemo::Assembly');
    boxNodes = boxes.nodes;
    assert_equal(all(arrayfun(@(node) isempty(node.compartment), boxNodes)), true, 'boxes name no compartment');
    folded = opensysml.renderView(viewModel, '#tree:RenderViewDemo::Assembly', 'compartments', 'most');
    foldedNodes = folded.nodes;
    sender = foldedNodes(strcmp({foldedNodes.name}, 'sender'));
    assert_equal(sender(1).compartment, 'parts', 'folded node names its compartment');
    assert_equal(numel(foldedNodes) < numel(boxNodes), true, 'folding draws fewer nodes');
    expect_identifier(@() opensysml.renderView(viewModel, '#tree:RenderViewDemo::Assembly', ...
        'compartments', 'rows'), 'opensysml:connect:invalidRequest', 'unknown compartments');
    fullView = opensysml.renderView(viewModel, 'RenderViewDemo::connections', 'ports', 'full');
    allPortNames = {};
    for n = 1:numel(fullView.nodes)
        allPortNames = [allPortNames, {fullView.nodes(n).ports.name}];
    end
    assert_equal(any(strcmp(allPortNames, 'spare')), true, 'RenderView full ports');
    importModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'import.sysml')), ...
        'name', 'import.sysml');
    imported = opensysml.importData(importModel, { ...
        struct('name', 'fleet.csv', 'content', sprintf('element,count,label\nV::vehicle,7,fast\nV::car,9,\n')), ...
        struct('name', 'labels.txt', 'content', '[{"element":"V::car","label":"slow"}]', 'format', 'json')});
    assert_equal(numel(imported.files), 2, 'ImportData files');
    assert_equal(imported.files{1}.name, 'fleet.csv', 'ImportData file name');
    assert_equal(imported.files{1}.values, 3, 'ImportData values');
    assert_equal(imported.files{1}.elements, 2, 'ImportData elements');
    assert_equal(imported.files{1}.assignments{1}.description, ...
        'V::vehicle::count = 7 (redefines V::Vehicle::count)', 'ImportData assignment');
    assert_equal(imported.files{1}.assignments{1}.redefines, 'V::Vehicle::count', 'ImportData redefines');
    assert_equal(imported.files{2}.assignments{1}.value, '"slow"', 'ImportData json value');
    assert_equal(isempty(strfind(imported.content, 'attribute :>> count = 7;')), false, 'ImportData content');
    assert_equal(numel(imported.documents), 1, 'ImportData documents');
    assert_equal(isempty(imported.modelHash), false, 'ImportData model hash');
    dry = importModel.importData({struct('name', 'fleet.tsv', 'content', sprintf('element\tcount\nV::vehicle\t7\n'))}, ...
        'dryRun', true);
    assert_equal(numel(dry.files{1}.assignments), 1, 'ImportData dry run assignment');
    asValues = importModel.importData({struct('name', 'fleet.tsv', 'content', sprintf('element\tcount\nV::vehicle\t7\n'))}, ...
        'importAs', 'values', 'dryRun', true);
    assert_equal(numel(asValues.files{1}.assignments), 1, 'ImportData importAs values');
    expect_identifier(@() importModel.importData({struct('name', 'fleet.tsv', 'content', sprintf('element\tcount\nV::vehicle\t7\n'))}, ...
        'importAs', 'rows', 'dryRun', true), 'opensysml:connect:invalidRequest', 'ImportData unknown importAs');
    assert_equal(isempty(dry.modelHash) && isempty(dry.content), true, 'ImportData dry run leaves the model');
    expect_identifier(@() opensysml.importData(importModel, ...
        {struct('name', 'fleet.csv', 'content', sprintf('element,count\nV::car,9\nV::Nowhere,1\n'))}), 'opensysml:', 'ImportData unknown element');
    expect_identifier(@() opensysml.importData(importModel, {}), 'opensysml:', 'ImportData nothing');

    probeModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'probe.sysml')), ...
        'name', 'probe.sysml');
    probeResults = ['{"source":"probe.xmi","configurations":[' ...
        '{"id":"_c1","name":"Cfg::''Group 1''","runs":2,"target":"target","behavior":"run","observables":["total"],' ...
        '"snapshots":[{"id":"_r1","values":{"total":4.0}},{"id":"_r2","values":{"total":6.0}}]},' ...
        '{"id":"_cn","name":"Cfg::Nothing","observables":[],"snapshots":[]}]}'];
    compared = opensysml.compareResults(probeModel, probeResults, 'seed', 7);
    assert_equal(compared.source, 'probe.xmi', 'CompareResults source');
    assert_equal(numel(compared.configurations), 2, 'CompareResults configurations');
    group = compared.configurations{1};
    assert_equal(group.name, 'Cfg::''Group 1''', 'CompareResults name');
    assert_equal(group.status, 'holds', 'CompareResults status');
    assert_equal([group.runs, group.completed, group.storedRuns], [2, 2, 2], 'CompareResults runs');
    assert_equal(group.seed, uint64(7), 'CompareResults seed');
    assert_equal(numel(group.rows), 2, 'CompareResults rows');
    total = group.observables{1};
    assert_equal(total.stored, 'total', 'CompareResults stored');
    assert_equal(total.feature, 'target.total', 'CompareResults feature');
    assert_equal(total.compared, true, 'CompareResults compared');
    assert_equal([total.tool.count, total.tool.mean, total.run.mean], [2, 5, 4], 'CompareResults distributions');
    assert_equal(total.difference.mean, '-20.0%', 'CompareResults difference');
    assert_equal(isempty(group.lines), false, 'CompareResults lines');
    assert_equal(compared.configurations{2}.status, 'unresolved', 'CompareResults unresolved');
    selected = probeModel.compareResults(probeResults, 'actions', {'Group 1'}, 'runs', 1, 'seed', 3);
    assert_equal(numel(selected.configurations), 1, 'CompareResults selected');
    assert_equal(selected.configurations{1}.runs, 1, 'CompareResults selected runs');
    expect_identifier(@() opensysml.compareResults(probeModel, probeResults, 'runs', -1), 'opensysml:', 'CompareResults negative runs');
    expect_identifier(@() opensysml.compareResults(probeModel, 'not json'), 'opensysml:', 'CompareResults malformed');

    runsModel = opensysml.parseSource(conn, fileread(fullfile(fixtures, 'monte_carlo.sysml')), ...
        'name', 'monte_carlo.sysml');
    settled = opensysml.runRuns(runsModel, 'action', 'MC::Settle', 'runs', 4, 'seed', 7, 'observe', {'t'});
    assert_equal(numel(settled.rows), 4, 'RunRuns rows');
    assert_equal(settled.observables, {'t'}, 'RunRuns observables');
    assert_equal(settled.distributions{1}.count, 4, 'RunRuns distribution count');
    assert_equal(settled.distributions{1}.numeric, true, 'RunRuns distribution numeric');
    assert_equal(settled.seed, uint64(7), 'RunRuns seed');
    sampled = runsModel.runRuns('analysis', 'MC::Mc', 'subject', 'MC::probe', 'runs', 3, 'seed', 1);
    assert_equal(numel(sampled.rows), 3, 'RunRuns sample rows');
    assert_equal(isempty(sampled.unconcluded), true, 'RunRuns concluded');
    assert_equal(isfield(sampled.outputs, 'Mean'), true, 'RunRuns Mean');
    expect_identifier(@() opensysml.runRuns(runsModel, 'action', 'MC::Settle', 'runs', 1), 'opensysml:', 'RunRuns unseeded');
    expect_identifier(@() opensysml.runRuns(runsModel, 'action', 'MC::Nope', 'runs', 1, 'draws', 'min'), 'opensysml:', 'RunRuns unknown');
    selfChecked = opensysml.selfCheck(runsModel);
    assert_equal(selfChecked.elements > 0 && selfChecked.checks > 0, true, 'SelfCheck counts');
    assert_equal(strncmp(selfChecked.summary, 'Self-model check: ', 18), true, 'SelfCheck summary');
    assert_equal(isempty(selfChecked.applications), false, 'SelfCheck applications');
    expect_identifier(@() runsModel.selfCheck('packages', {'Nope'}), 'opensysml:', 'SelfCheck unknown package');
    recorded = opensysml.recordRun(runsModel, 'MC::Cost', 'subject', 'MC::p');
    assert_equal(isa(recorded.run, 'opensysml.AnalysisResult'), true, 'RecordRun run');
    assert_equal(isempty(recorded.run.outputs), false, 'RecordRun run outputs');
    assert_equal(recorded.package, 'Records', 'RecordRun package');
    assert_equal(recorded.records, {'Records::Cost_run1'}, 'RecordRun records');
    assert_equal(numel(recorded.documents), 1, 'RecordRun documents');
    assert_equal(isempty(recorded.modelHash), false, 'RecordRun model hash');
    sampledRecord = runsModel.recordRun('MC::Mc', 'subject', 'MC::probe', 'runs', 2, 'seed', 3);
    assert_equal(isempty(sampledRecord.sample), false, 'RecordRun sample');
    assert_equal(numel(sampledRecord.records), 3, 'RecordRun sample records');
    expect_identifier(@() opensysml.recordRun(runsModel, 'MC::Nope'), 'opensysml:', 'RecordRun unknown');

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
    drawSource = strjoin({ ...
        'package test {', ...
        '  private import ScalarValues::*;', ...
        '  private import RandomFunctions::*;', ...
        '  action draw {', ...
        '    out d : Real[1] = uniform(0.0, 1.0);', ...
        '    out n : Integer[1] = uniformInteger(1, 6);', ...
        '  }', ...
        '}'}, sprintf('\n'));
    drawing = opensysml.parseSource(conn, drawSource, 'name', 'draw.sysml');
    drawn = opensysml.executeAction(drawing, 'test::draw', 'draws', 'min', 'seed', 7);
    assert_equal(drawn.outputs.d, 0, 'executeAction draws min');
    assert_equal(drawn.outputs.n, int64(1), 'executeAction draws min integer');
    maxDrawn = drawing.executeAction('test::draw', 'draws', 'max');
    assert_equal(maxDrawn.outputs.n, int64(6), 'executeAction draws max');
    seededOnce = opensysml.executeAction(drawing, 'test::draw', 'seed', 7);
    seededTwice = opensysml.executeAction(drawing, 'test::draw', 'seed', 7);
    assert_equal(seededOnce.outputs.d, seededTwice.outputs.d, 'executeAction seed repeats');
    delayedSource = strjoin({ ...
        'package test {', ...
        '  private import ScalarValues::*;', ...
        '  action delayed {', ...
        '    attribute count : Integer = 0;', ...
        '    first start;', ...
        '    then action wait accept after 5 [SI::s];', ...
        '    then action tick assign count := count + 1;', ...
        '    then done;', ...
        '  }', ...
        '}'}, sprintf('\n'));
    delayed = opensysml.parseSource(conn, delayedSource, 'name', 'delayed.sysml');
    stepped = opensysml.executeAction(delayed, 'test::delayed', 'clockStep', 4);
    assert_equal(stepped.outputs.count, int64(1), 'executeAction clockStep');
    assert_equal(stepped.finalTime, 8, 'executeAction clockStep final time');
    advanced = opensysml.executeAction(delayed, 'test::delayed', 'advance', 10);
    assert_equal(advanced.outputs.count, int64(1), 'executeAction advance');
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
    bound = opensysml.parseSource(conn, sprintf([ ...
        'package Bound {\n' ...
        '    private import ScalarValues::*;\n' ...
        '    part def Thing { attribute v : Integer = 2; }\n' ...
        '    part t : Thing;\n' ...
        '    requirement def Under {\n' ...
        '        subject s : Thing;\n' ...
        '        in limit : Integer;\n' ...
        '        in slack : Integer = 0;\n' ...
        '        require constraint { s.v + slack < limit }\n' ...
        '    }\n' ...
        '    constraint def Between { in low : Integer; in high : Integer = 10; low < high }\n' ...
        '}\n']), 'name', 'bound.sysml');
    boundNamed = bound.verifyRequirement('Bound::Under', 'subject', 'Bound::t', ...
        'namedArguments', struct('limit', int64(5)));
    assert_equal(boundNamed.holds, true, 'verifyRequirement namedArguments');
    boundPositional = opensysml.verifyRequirement(bound, 'Bound::Under', ...
        'subject', 'Bound::t', 'arguments', {int64(5), int64(4)});
    assert_equal(boundPositional.holds, false, 'verifyRequirement arguments');
    boundConstraint = bound.verifyConstraint('Bound::Between', 'arguments', {int64(3)});
    assert_equal(boundConstraint.holds, true, 'verifyConstraint arguments');
    boundMap = bound.verifyConstraint('Bound::Between', ...
        'namedArguments', containers.Map({'low'}, {int64(3)}));
    assert_equal(boundMap.holds, true, 'verifyConstraint namedArguments as a Map');
    boundMapFails = bound.verifyConstraint('Bound::Between', ...
        'namedArguments', containers.Map({'low'}, {int64(12)}));
    assert_equal(boundMapFails.holds, false, 'verifyConstraint Map arguments reach the check');
    assert_error(@() bound.verifyConstraint('Bound::Between', 'namedArguments', 42), ...
        'opensysml:argument', 'verifyConstraint refuses a non-struct namedArguments');
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
    % `out x : Real` states no multiplicity, so the analysis holds x in its
    % effective [0..*]: a one-element sequence, decoded as a cell.
    assert_equal(result.outputs.x, {3}, 'runAnalysis');
    assert_equal(resultMethod.outputs.x, {3}, 'Model.runAnalysis');
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

function codes = diagnosticCodes(diags)
    if isstruct(diags), diags = num2cell(diags(:)'); end
    codes = cell(1, numel(diags));
    for i = 1:numel(diags)
        d = diags{i};
        if isstruct(d) && isfield(d, 'code'), codes{i} = char(d.code); else, codes{i} = ''; end
    end
end
