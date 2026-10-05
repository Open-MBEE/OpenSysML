function test_errors_capabilities()
%TEST_ERRORS_CAPABILITIES Check typed errors and service capability handling.

    caps = opensysml.capabilities();
    assert_equal(caps.parseSources, 'parse_sources', 'parseSources capability');
    assert_equal(caps.verificationQuestions, 'verification_questions', 'question capability');
    assert_equal(caps.undeterminedValue, 'undetermined_value', 'undetermined capability');
    assert_equal(numel(fieldnames(caps)) >= 50, true, 'exhaustive capability list');

    conn = opensysml.external('127.0.0.1:1');
    conn.primeServerInfo(struct('version', 'v1', ...
        'capabilities', {{'query', 'parse_sources'}}));
    assert_equal(conn.hasCapability('query'), true, 'reported capability');
    assert_equal(conn.hasCapability('convert'), false, 'absent capability');
    text = conn.describe();
    assert_equal(~isempty(strfind(text, '127.0.0.1:1 (version v1, capabilities: parse_sources, query)')), ...
        true, 'sorted service description');

    expect_error(@() conn.require('convert'), 'opensysml:missingCapability');
    err = opensysml.lastError();
    assert_equal(err.identifier, 'opensysml:missingCapability', 'lastError identifier');
    assert_equal(err.details.capability, 'convert', 'lastError capability');
    assert_equal(err.details.service, text, 'lastError service');
    assert_equal(~isempty(strfind(err.message, 'GetServerInfo reports ''convert''')), ...
        true, 'missing capability remedy');
    expect_error(@() opensysml.listEngines(conn), 'opensysml:missingCapability');
    err = opensysml.lastError();
    assert_equal(err.details.capability, 'engines', 'RPC capability gate');

    expect_error(@() opensysml.internal.connectError('invalid_argument', ...
        'bad request', 400, 'ParseSources', conn, {}, struct()), ...
        'opensysml:connect:invalidRequest');
    err = opensysml.lastError();
    assert_equal(err.details.code, 'invalid_argument', 'Connect code');
    assert_equal(err.details.httpStatus, 400, 'Connect status');

    expect_error(@() opensysml.internal.connectError('not_found', ...
        'symbol not found: Missing::Symbol', 404, 'GetSymbol', conn, {}, struct()), ...
        'opensysml:connect:symbolNotFound');

    answer = struct('error', 'model execution failed', ...
        'diagnostics', struct('severity', 'error', 'message', 'bad expression'));
    expect_error(@() opensysml.internal.checkError(answer, 'Evaluate'), ...
        'opensysml:diagnostics:execution');
    err = opensysml.lastError();
    assert_equal(numel(err.diagnostics), 1, 'diagnostics side channel');

    traceEvent = opensysml.internal.decodeDocumentValue(struct('event', ...
        struct('kind', 'entry', 'time', struct('realValue', 1.5), ...
        'state', 'active', 'text', 'enter: active')));
    details = struct('trace', {{traceEvent}}, 'traceDropped', 2);
    expect_error(@() opensysml.internal.checkError( ...
        struct('error', 'state machine failed'), 'ExecuteState', details), ...
        'opensysml:diagnostics:execution');
    err = opensysml.lastError();
    assert_equal(err.details.trace{1}.kind, 'entry', 'failed state trace');
    assert_equal(err.details.trace{1}.state, 'active', 'failed state trace state');
    assert_equal(err.details.traceDropped, 2, 'failed state trace dropped count');

    old = opensysml.external('old.invalid:1');
    old.primeServerInfo(struct('answered', false));
    assert_equal(old.hasCapability('query'), false, 'old server has no capabilities');
    assert_equal(~isempty(strfind(old.describe(), 'too old to answer GetServerInfo')), ...
        true, 'old server description');

    mismatch = opensysml.external('mismatch.invalid:1');
    mismatch.primeServerInfo(struct('version', 'v1', 'capabilities', {{'query'}}));
    mismatch.expectedVersion = 'v2';
    expect_error(@() mismatch.primeServerInfo( ...
        struct('version', 'v1', 'capabilities', {{'query'}})), ...
        'opensysml:staleService');
    err = opensysml.lastError();
    assert_equal(isfield(err.details, 'service'), true, ...
        'stale service details');

    fprintf('errors and capabilities ok\n');
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
