function test_wire()
%TEST_WIRE parseInt64 range rules, the curl address guard, and the
%map<string,Value> decode that must not trip the single-key heuristic.

    assert_equal(opensysml.parseInt64('007'), int64(7), 'leading zeros');
    assert_equal(opensysml.parseInt64('-9223372036854775808'), min_int64(), 'int64 min exact');
    assert_error(@() opensysml.parseInt64('9223372036854775808'), 'opensysml:transport', 'int64 max+1');
    assert_error(@() opensysml.parseInt64('-9223372036854775809'), 'opensysml:transport', 'int64 min-1');
    assert_error(@() opensysml.parseInt64(''), 'opensysml:transport', 'empty literal');
    assert_error(@() opensysml.parseInt64('-'), 'opensysml:transport', 'sign only');

    % an output named like a Value arm decodes as a map entry, not a Value
    out = opensysml.internal.decodeValueMap(struct('intValue', struct('intValue', '7')));
    assert_equal(out.intValue, int64(7), 'arm-named map key');
    out = opensysml.internal.decodeValueMap(struct('x', struct('stringValue', 'y')));
    assert_equal(out.x, 'y', 'plain map key');

    % the Octave transport must refuse an address carrying shell metacharacters
    if ~exist('matlab.net.http.RequestMessage', 'class')
        conn = opensysml.external('http://x$(id)/');
        assert_error(@() opensysml.call(conn, 'GetServerInfo', struct()), 'opensysml:transport', 'metacharacters');
    end
    fprintf('wire ok\n');
end

function m = min_int64()
    m = intmin('int64');
end
