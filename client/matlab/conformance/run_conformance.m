function run_conformance(varargin)
%RUN_CONFORMANCE Drive every scenario through the client's callRaw, the
%whole RPC surface with no subset skips; --binary|--address selects the service.

    args = varargin;
    if isempty(args) && ~isempty(getenv('RUN_CONFORMANCE_ARGS'))
        args = strsplit(getenv('RUN_CONFORMANCE_ARGS'));
    end
    opts = conf_args(args);

    here = fileparts(mfilename('fullpath'));
    root = fileparts(fileparts(fileparts(here)));   % conformance/ -> client/matlab -> client -> repo
    if isempty(opts.scenarios), opts.scenarios = fullfile(root, 'conformance', 'scenarios'); end
    if isempty(opts.fixtures), opts.fixtures = fullfile(root, 'conformance', 'fixtures'); end

    scenarios = load_scenarios(opts.scenarios);

    if ~isempty(opts.address)
        conn = opensysml.external(opts.address);
        serviceName = opts.address;
        isPrivate = false;
    else
        bin = opts.binary;
        if isempty(bin)
            bin = getenv('OPENSYSML_GRPC_BINARY');
        end
        if isempty(bin), bin = opensysml.resolveBinary(); end
        conn = opensysml.private('binary', bin);
        serviceName = bin;
        isPrivate = true;
    end

    try
        [~, capabilities] = conn.serverInfo();
        capabilities = sort(capabilities);
        models = containers.Map();   % model spec -> hash
        results = {};
        for i = 1:numel(scenarios)
            sc = scenarios{i};
            if ~isempty(opts.run) && isempty(strfind(sc('id'), opts.run))
                continue;
            end
            result = run_scenario(sc, conn, opts.fixtures, models, capabilities);
            print_result(result, opts.verbose);
            results{end+1} = result;
        end

        counts = struct('pass', 0, 'fail', 0, 'skip', 0, 'error', 0);
        for i = 1:numel(results)
            o = results{i}('outcome');
            counts.(o) = counts.(o) + 1;
        end

        summary = any_map();
        summary('protocol') = 'connect-json';
        summary('service') = serviceName;
        summary('capabilities') = capabilities;
        summary('total') = numel(results);
        summary('passed') = counts.pass;
        summary('failed') = counts.fail;
        summary('skipped') = counts.skip;
        summary('errored') = counts.error;
        summary('results') = results;
        report = any_map();
        report('service') = serviceName;
        report('total') = numel(results);
        report('passed') = counts.pass;
        report('failed') = counts.fail;
        report('skipped') = counts.skip;
        report('errored') = counts.error;
        report('protocols') = {summary};
        fprintf('total=%d passed=%d failed=%d skipped=%d errored=%d\n', ...
                numel(results), counts.pass, counts.fail, counts.skip, counts.error);
        if ~isempty(opts.report)
            text = [json_write(report) sprintf('\n')];
            if strcmp(opts.report, '-')
                fprintf('%s', text);
            else
                fid = fopen(opts.report, 'w');
                fwrite(fid, text);
                fclose(fid);
            end
        end

        if isPrivate, conn.close(); end
        if counts.fail > 0 || counts.error > 0
            exit_code(1, sprintf('%d failed or errored scenarios', counts.fail + counts.error));
        end
        if counts.skip > 0 && ~opts.allow_skips
            exit_code(1, sprintf('%d scenarios skipped for missing capabilities; use --allow-skips', counts.skip));
        end
    catch e
        if isPrivate, try, conn.close(); catch, end, end
        exit_code(1, e.message);
    end
end

function exit_code(code, message)
    if ~isempty(message), fprintf(2, 'conformance: %s\n', message); end
    if code ~= 0, exit(code); end
end

function result = run_scenario(sc, conn, fixturesDir, models, capabilities)
    started = tic;
    result = any_map();
    id = sc('id');
    rpc = sc('rpc');
    slash = strfind(rpc, '/');
    if ~isempty(slash), method = rpc(slash(end)+1:end); else, method = rpc; end
    result('id') = id;
    result('outcome') = 'pass';
    result('rpc') = method;
    result('status') = 'OK';

    expect = sc('expect');
    missing = {};
    if isKey(sc, 'requires_capabilities')
        for i = 1:numel(sc('requires_capabilities'))
            cap = sc('requires_capabilities'){i};
            if ~any(strcmp(capabilities, cap)), missing{end+1} = cap; end
        end
    end
    if ~isempty(missing)
        if isKey(sc, 'expect_without_capability')
            expect = sc('expect_without_capability');
        else
            result('outcome') = 'skip';
            result('status') = '-';
            result('reason') = ['missing capability ' strjoin(missing, ', ')];
            result('duration_ms') = toc(started) * 1000;
            return;
        end
    end

    modelHash = '';
    if isKey(sc, 'model') && ~isempty(sc('model'))
        try
            modelHash = model_hash(sc('model'), conn, fixturesDir, models);
        catch e
            result('outcome') = 'error';
            result('status') = '-';
            result('reason') = e.message;
            result('duration_ms') = toc(started) * 1000;
            return;
        end
    end

    request = sc('request');
    try
        request = conf_placeholders(request, modelHash, fixturesDir);
    catch e
        result('outcome') = 'error';
        result('status') = '-';
        result('reason') = e.message;
        result('duration_ms') = toc(started) * 1000;
        return;
    end

    try
        [status, contentType, body] = opensysml.callRaw(conn, method, json_write(request));
    catch e
        result('outcome') = 'error';
        result('status') = '-';
        result('reason') = e.message;
        result('duration_ms') = toc(started) * 1000;
        return;
    end

    wantStatus = 'OK';
    if isKey(expect, 'status') && ~isempty(expect('status')), wantStatus = expect('status'); end
    isJson = ~isempty(strfind(lower(contentType), 'application/json'));
    failures = {};
    if status ~= 200 && isJson
        out = json_tree(body);
        code = 'unknown'; message = '';
        if isKey(out, 'code'), code = out('code'); end
        if isKey(out, 'message'), message = out('message'); end
        result('status') = status_canonical(code);
        if ~strcmp(result('status'), wantStatus)
            result('outcome') = 'fail';
            failures{end+1} = sprintf('status: %s (%s), want %s', result('status'), message, wantStatus);
        elseif isKey(expect, 'status_message_contains') && isempty(strfind(message, expect('status_message_contains')))
            result('outcome') = 'fail';
            failures{end+1} = sprintf('status message "%s" does not contain "%s"', message, expect('status_message_contains'));
        end
    elseif status ~= 200 || ~isJson
        result('outcome') = 'error';
        result('status') = '-';
        result('reason') = sprintf('transport answered HTTP %d with a non-JSON body', status);
    else
        actual = json_tree(body);
        actual = conf_normalize(actual, modelHash);
        actual = conf_label_ids(actual);
        if ~strcmp(wantStatus, 'OK')
            result('outcome') = 'fail';
            failures{end+1} = sprintf('the call succeeded, want status %s', wantStatus);
        else
            failures = conf_compare(expect, actual);
            if ~isempty(failures), result('outcome') = 'fail'; end
        end
    end
    if ~isempty(failures), result('failures') = failures; end
    result('duration_ms') = toc(started) * 1000;
end

function hash = model_hash(spec, conn, fixturesDir, models)
    % one parse per distinct model spec, keyed on its fields' JSON
    key = json_write(spec);
    if isKey(models, key)
        hash = models(key); return;
    end
    language = '';
    strict = false;
    if isKey(spec, 'language'), language = spec('language'); end
    if isKey(spec, 'strict_conformance'), strict = spec('strict_conformance'); end
    if isKey(spec, 'fixture') && ~isempty(spec('fixture'))
        name = spec('fixture');
        request = any_map();
        request('content') = fileread(conf_fixture_path(fixturesDir, name));
        if ~isempty(language), request('language') = language; end
        if strict, request('strictConformance') = true; end
        method = 'ParseFile';
    else
        docs = {};
        fixtures = {};
        if isKey(spec, 'fixtures'), fixtures = spec('fixtures'); end
        for i = 1:numel(fixtures)
            f = fixtures{i};
            doc = any_map();
            doc('name') = f;
            doc('content') = fileread(conf_fixture_path(fixturesDir, f));
            if ~isempty(language), doc('language') = language; end
            docs{end+1} = doc;
        end
        request = any_map();
        request('documents') = docs;
        if strict, request('strictConformance') = true; end
        method = 'ParseSources';
    end
    [status, contentType, body] = opensysml.callRaw(conn, method, json_write(request));
    isJson = ~isempty(strfind(lower(contentType), 'application/json'));
    if status ~= 200 || ~isJson
        error('opensysml:conformance', 'model setup %s answered HTTP %d', method, status);
    end
    answer = json_tree(body);
    hash = answer('modelHash');
    models(key) = hash;
end

function m = any_map()
    % an any-valued map: uniform-value maps refuse the runner's mixed
    % assignments (numbers, chars, cells, nested maps)
    m = containers.Map('KeyType', 'char', 'ValueType', 'any');
end

function print_result(result, verbose)
    switch result('outcome')
        case 'pass', mark = 'PASS';
        case 'fail', mark = 'FAIL';
        case 'skip', mark = 'SKIP';
        otherwise, mark = 'ERR ';
    end
    fprintf('%s %-46s %s\n', mark, result('id'), result('status'));
    if isKey(result, 'reason'), fprintf('       %s\n', result('reason')); end
    if isKey(result, 'failures')
        fs = result('failures');
        for i = 1:numel(fs), fprintf('       %s\n', fs{i}); end
    end
    if verbose, fprintf('       duration_ms=%.3f\n', result('duration_ms')); end
end
