function test_live()
%TEST_LIVE Live calls against a real service: $OPENSYSML_SERVICE must be
%set (external connection); otherwise the group skips.

    addr = getenv('OPENSYSML_SERVICE');
    if isempty(addr)
        fprintf('live: skipped (OPENSYSML_SERVICE unset)\n');
        return;
    end
    conn = opensysml.connect();

    [version, capabilities] = conn.serverInfo();
    assert_equal(ischar(version) && ~isempty(version), true, 'serverInfo version');
    assert_equal(iscell(capabilities), true, 'serverInfo capabilities');
    assert_equal(conn.hasCapability('query'), true, 'hasCapability');
    assert_equal(conn.hasCapability('nonsense_capability'), false, 'hasCapability negative');

    fixture = fullfile(fileparts(mfilename('fullpath')), '..', '..', '..', 'conformance', 'fixtures', 'simple_part.sysml');
    model = opensysml.parseSource(conn, fileread(fixture), 'name', 'simple_part.sysml');
    assert_equal(ischar(model.hash) && ~isempty(model.hash), true, 'parseSource hash');

    sym = opensysml.symbol(model, 'Test::SimplePart');
    assert_equal(isstruct(sym), true, 'symbol is a record');

    v = opensysml.evaluate(model, '2 + 2');
    assert_equal(v, int64(4), 'evaluate 2+2');

    inst = opensysml.instantiate(model, 'Test::SimplePart');
    assert_equal(isa(inst.id, 'int64'), true, 'instance id int64');
    assert_equal(inst.type_symbol_id, 'Test::SimplePart', 'instance type');
    assert_equal(isa(inst.feature_values, 'containers.Map'), true, 'feature_values map');

    bmodel = opensysml.parseSource(conn, fileread(fullfile(fileparts(fixture), 'behavior.sysml')), 'name', 'behavior.sysml');
    qmodel = opensysml.parseSource(conn, fileread(fullfile(fileparts(fixture), 'query.sysml')), 'name', 'query.sysml');

    ran = opensysml.executeAction(bmodel, 'Test::addFive', 'inputs', struct('result', int64(10)));
    assert_equal(ran.outputs.result, int64(15), 'executeAction 10 -> 15');

    states = opensysml.executeState(bmodel, 'Test::Machine');
    assert_equal(states.statesVisited, {'init'; 'Running'; 'done'}, 'executeState visited');

    rows = opensysml.query(qmodel, 'oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name');
    assert_equal(numel(rows.elements), 3, 'query elements');

    assert_error(@() opensysml.evaluate(model, '1 +'), 'opensysml:diagnostics', 'bad expression -> diagnostics');
    assert_error(@() opensysml.symbol(struct_hash_model(conn), 'No::Such'), 'opensysml:connect', 'stale/bad hash -> connect error');

    m2 = opensysml.parseSources(conn, {{'a.sysml', 'package A { part def P; }'}, {'b.sysml', 'package B { part def Q; }'}});
    assert_equal(~isempty(m2.hash), true, 'parseSources');
    fprintf('live ok\n');
end

function m = struct_hash_model(conn)
    m = opensysml.Model(conn, 'nosuchhash0000000000', {});
end
