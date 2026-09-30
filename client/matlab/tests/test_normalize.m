function test_normalize()
%TEST_NORMALIZE The response normalization rules as unit tests.

    addpath(fullfile(fileparts(mfilename('fullpath')), '..', 'conformance', 'private'));

    % version -> ${version} only inside a server-info shaped object
    m = kv_map();
    m('version') = '1.2.3'; m('capabilities') = {'a'};
    m = conf_normalize(m, '');
    assert_equal(m('version'), '${version}', 'version marker');
    m = kv_map();
    m('version') = '1.2.3';
    m = conf_normalize(m, '');
    assert_equal(m('version'), '1.2.3', 'plain version untouched');

    % the model hash -> ${model_hash}
    m = kv_map();
    m('h') = 'deadbeef';
    m = conf_normalize(m, 'deadbeef');
    assert_equal(m('h'), '${model_hash}', 'model hash marker');

    % absolute paths -> ${path}
    m = kv_map();
    m('f') = '/tmp/x.sysml';
    m = conf_normalize(m, '');
    assert_equal(m('f'), '${path}', 'path marker');

    % self_id of 0/"0" is removed
    m = kv_map();
    m('selfId') = '0';
    m = conf_normalize(m, '');
    assert_equal(isKey(m, 'selfId'), false, 'selfId 0 removed');

    % instance ids -> @1, @2, ... consistently within one response
    inst = kv_map();
    inst('id') = 41;
    inst('typeSymbolId') = 'A::b';
    inst('featureValues') = kv_map();
    other = kv_map();
    other('instanceId') = 41;
    top = kv_map();
    top('i') = inst; top('other') = other;
    top = conf_normalize(top, '');
    top = conf_label_ids(top);
    assert_equal(top('i')('id'), '@1', 'instance id label');
    assert_equal(top('other')('instanceId'), '@1', 'same id same label');
    a = kv_map(); a('instanceId') = 1;
    b = kv_map(); b('instanceId') = 2;
    m2 = kv_map();
    m2('a') = a; m2('b') = b;
    m2 = conf_label_ids(m2);
    assert_equal(m2('a')('instanceId'), '@1', 'first id @1');
    assert_equal(m2('b')('instanceId'), '@2', 'second id @2');
    fprintf('normalize ok\n');
end

function m = kv_map()
    % an any-valued map: constructor form infers a uniform value type and
    % the runner writes @N labels into numeric-valued maps
    m = containers.Map('KeyType', 'char', 'ValueType', 'any');
end
