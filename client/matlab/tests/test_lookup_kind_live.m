function test_lookup_kind_live()
%TEST_LOOKUP_KIND_LIVE An id the service reports names its element where a name is
%spelled alike; needs $OPENSYSML_SERVICE, otherwise the group skips.

    if isempty(getenv('OPENSYSML_SERVICE'))
        fprintf('lookup kind live: skipped (OPENSYSML_SERVICE unset)\n');
        return;
    end
    conn = opensysml.connect();
    fixture = fullfile(fileparts(mfilename('fullpath')), '..', '..', '..', 'conformance', ...
        'fixtures', 'lookup_kind.sysml');
    model = opensysml.parseSource(conn, fileread(fixture), 'name', 'lookup_kind.sysml');
    assert_equal(conn.hasCapability('symbol_lookup_kind'), true, 'symbol_lookup_kind announced');

    % W holds an unnamed third member, whose id is spelled as its sibling '@2' is named.
    w = model.symbol('Col::W');
    reported = w.record.childIds;
    children = w.children();
    ids = cellfun(@(c) c.id, children, 'UniformOutput', false);
    assert_equal(ids(:), reported(:), 'children are the elements childIds reports');
    grandchildren = {};
    for i = 1:numel(children)
        inner = children{i}.children();
        for j = 1:numel(inner), grandchildren{end+1} = inner{j}.name; end
    end
    assert_equal(sort(grandchildren(:)), {'inner'; 'named'}, 'each member keeps its own subtree');

    % The connector end's generated id spells the package Col__V__c_pend0.
    name = 'Col__V__c_pend0';
    byName = opensysml.getSymbol(model, name, 'lookup', 'qualified_name');
    assert_equal(byName.kind, 'package', 'qualified name reads the package');
    byId = opensysml.getSymbol(model, name, 'lookup', 'element_id');
    assert_equal(strcmp(byId.kind, 'package'), false, 'element id reads the connector end');
    legacy = opensysml.getSymbol(model, name);
    assert_equal(legacy.kind, 'package', 'legacy form reads the name first');
    found = model.find(byName.id);
    assert_equal(found.kind, 'package', 'find reads the id the package is reported under');

    viaId = opensysml.elementProperties(model, name, 'owner', struct('lookup', 'element_id'));
    viaName = opensysml.elementProperties(model, name, 'owner', struct('lookup', 'qualified_name'));
    assert_equal(strcmp(viaId.element.id, viaName.element.id), false, ...
        'elementProperties reads the stated kind');
    expect_argument_error(@() opensysml.getSymbol(model, name, 'lookup', 'nonsense'));
end

function expect_argument_error(fn)
    try
        fn();
    catch e
        assert_equal(e.identifier, 'opensysml:argument', 'an unknown lookup kind is refused');
        return;
    end
    error('test:noError', 'an unknown lookup kind was accepted');
end
