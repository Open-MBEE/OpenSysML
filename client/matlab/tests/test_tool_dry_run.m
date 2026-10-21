function test_tool_dry_run()
%TEST_TOOL_DRY_RUN Check the typed tool dry-run preview decoder and its place
%on AnalysisResult, offline.

    raw = struct('action', 'Demo::compute', 'tool', 'octave', 'version', '8.4', ...
        'manifest', '/tools/octave.json', 'executable', '/usr/bin/octave', ...
        'protocol', 'stdio', 'args', {{'--eval', 'run'}}, 'env', {{'HOME=/tmp'}}, ...
        'cwd', '/tmp/run', 'stdin', 'data', 'stdinData', 'x = 1', ...
        'inputFile', struct('name', 'in.json', 'format', 'json', 'data', '{}'), ...
        'outputDir', true, ...
        'inputs', {{struct('variable', 'x', 'parameter', 'p', 'value', '250.0', 'unit', 'm')}}, ...
        'outputs', {{struct('variable', 'y', 'parameter', 'q')}}, ...
        'replySource', 'stdout', 'lines', {{'would run /usr/bin/octave'}});
    preview = opensysml.internal.decodeToolDryRun(raw);
    assert_equal(preview.action, 'Demo::compute', 'dry run action');
    assert_equal(preview.tool, 'octave', 'dry run tool');
    assert_equal(preview.version, '8.4', 'dry run version');
    assert_equal(preview.args, {'--eval', 'run'}, 'dry run args');
    assert_equal(preview.env, {'HOME=/tmp'}, 'dry run env');
    assert_equal(preview.inputFile.name, 'in.json', 'dry run input file');
    assert_equal(preview.outputDir, true, 'dry run output dir');
    assert_equal(numel(preview.inputs), 1, 'dry run inputs');
    assert_equal(preview.inputs{1}.value, '250.0', 'dry run input value');
    assert_equal(preview.inputs{1}.unit, 'm', 'dry run input unit');
    assert_equal(preview.outputs{1}.parameter, 'q', 'dry run output parameter');
    assert_equal(preview.replySource, 'stdout', 'dry run reply source');
    assert_equal(preview.lines, {'would run /usr/bin/octave'}, 'dry run lines');

    bare = opensysml.internal.decodeToolDryRun(struct('tool', 't'));
    assert_equal(isempty(bare.inputFile), true, 'bare dry run input file');
    assert_equal(bare.args, {}, 'bare dry run args');
    assert_equal(bare.outputDir, false, 'bare dry run output dir');

    conn = opensysml.external('127.0.0.1:1');
    conn.primeServerInfo(struct('version', 'test', 'capabilities', {{'verification'}}));
    model = opensysml.Model(conn, 'offline-hash', {});
    answer = struct('toolDryRun', raw);
    result = opensysml.internal.decodeAnalysisResult(model, answer);
    assert_equal(isa(result, 'opensysml.AnalysisResult'), true, 'dry run result class');
    assert_equal(result.toolDryRun.tool, 'octave', 'dry run on the result');
    assert_equal(isempty(fieldnames(result.outputs)), true, 'dry run has no outputs');
    plain = opensysml.internal.decodeAnalysisResult(model, struct());
    assert_equal(isempty(plain.toolDryRun), true, 'a real run has no preview');
    fprintf('tool dry run ok\n');
end
