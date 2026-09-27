function test_runner_rules()
%TEST_RUNNER_RULES The conformance comparison rules as unit tests, over
%the same json_tree values the runner works on.

    addpath(fullfile(fileparts(mfilename('fullpath')), '..', 'conformance', 'private'));

    % tolerance boundaries: 1e-9 relative both ways
    act = @(x) map_val('v', x);
    assert_equal(has_fail(conf_compare(expect_response(map_val('v', 1.0)), act(1.0 + 9e-10))), false, 'inside tolerance');
    assert_equal(has_fail(conf_compare(expect_response(map_val('v', 1.0)), act(1.0 + 2e-9))), true, 'outside tolerance');

    % expected integer vs int64 wire string
    assert_equal(has_fail(conf_compare(expect_response(map_val('v', 7)), act('7'))), false, 'int string');
    assert_equal(has_fail(conf_compare(expect_response(map_val('v', 7)), act('8'))), true, 'int string mismatch');

    % expected snake_case key against lowerCamel wire answer
    expect = expect_response(map_kv('model_hash', 'h'));
    assert_equal(has_fail(conf_compare(expect, map_kv('modelHash', 'h'))), false, 'snake->camel');

    % "0" reads as the default 0
    [got, found] = conf_lookup(map_kv('x', '0'), 'x');
    assert_equal(found && conf_is_default(got), true, '"0" is default');
    assert_equal(conf_is_default('1'), false, '"1" is not default');

    % list length must match exactly
    assert_equal(has_fail(conf_compare(expect_response(map_val('v', {1, 2})), map_val('v', {1, 2, 3}))), true, 'list length');
    assert_equal(has_fail(conf_compare(expect_response(map_val('v', {1, 2})), map_val('v', {1, 2}))), false, 'list equal');

    % absent field satisfied by all-default expectation
    assert_equal(has_fail(conf_compare(expect_response(map_kv('v', false)), containers.Map())), false, 'absent default');
    assert_equal(has_fail(conf_compare(expect_response(map_kv('v', 1)), containers.Map())), true, 'absent non-default');

    % contains_all over a wildcard path
    a = map_kv('rows', {map_kv('name', 'x'), map_kv('name', 'y')});
    e = containers.Map(); e('contains_all') = containers.Map({'rows.*.name'}, {{'x', 'y'}});
    assert_equal(has_fail(conf_compare(e, a)), false, 'contains_all wildcard');
    e = containers.Map(); e('contains_all') = containers.Map({'rows.*.name'}, {{'z'}});
    assert_equal(has_fail(conf_compare(e, a)), true, 'contains_all missing');

    % counts and min_counts; absent path counts as zero
    e = containers.Map(); e('counts') = containers.Map({'rows'}, {2});
    assert_equal(has_fail(conf_compare(e, a)), false, 'counts exact');
    e = containers.Map(); e('counts') = containers.Map({'rows'}, {3});
    assert_equal(has_fail(conf_compare(e, a)), true, 'counts wrong');
    e = containers.Map(); e('min_counts') = containers.Map({'rows'}, {2});
    assert_equal(has_fail(conf_compare(e, a)), false, 'min_counts');
    e = containers.Map(); e('counts') = containers.Map({'missing'}, {0});
    assert_equal(has_fail(conf_compare(e, a)), false, 'absent counts 0');

    % non_empty and absent
    e = containers.Map(); e('non_empty') = {'v'};
    assert_equal(has_fail(conf_compare(e, act('x'))), false, 'non_empty');
    e = containers.Map(); e('non_empty') = {'missing'};
    assert_equal(has_fail(conf_compare(e, act('x'))), true, 'non_empty absent');
    e = containers.Map(); e('absent') = {'v'};
    assert_equal(has_fail(conf_compare(e, act(''))), false, 'absent empty ok');
    assert_equal(has_fail(conf_compare(e, act('x'))), true, 'absent non-default');

    % status canonicalization
    assert_equal(status_canonical('invalid_argument'), 'INVALID_ARGUMENT', 'status invalid_argument');
    assert_equal(status_canonical('OK'), 'OK', 'status OK');
    fprintf('runner_rules ok\n');
end

function m = map_val(key, v)
    m = containers.Map({key}, {v});
end

function m = map_kv(key, v)
    m = containers.Map({key}, {v});
end

function e = expect_response(response)
    e = containers.Map({'response'}, {response});
end

function tf = has_fail(failures)
    tf = ~isempty(failures);
end
