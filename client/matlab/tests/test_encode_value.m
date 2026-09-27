function test_encode_value()
%TEST_ENCODE_VALUE The request encodings and their decode round-trips.

    assert_equal(opensysml.encodeValue(true), struct('boolValue', true), 'bool');
    assert_equal(opensysml.encodeValue(int64(7)), struct('intValue', '7'), 'int64');
    assert_equal(opensysml.encodeValue(1.5), struct('realValue', 1.5), 'real');
    assert_equal(opensysml.encodeValue('x'), struct('stringValue', 'x'), 'string');
    assert_equal(opensysml.encodeValue([]), struct('null', ''), 'null');

    c = opensysml.encodeValue(complex(1, -2));
    assert_equal(c.complex.real, 1, 'complex real');
    assert_equal(c.complex.imaginary, -2, 'complex imag');
    assert_equal(opensysml.decodeValue(c), complex(1, -2), 'complex round trip');

    v = opensysml.encodeValue(struct('instanceRef', int64(9)));
    assert_equal(v.instanceId, '9', 'instanceRef');
    dv = opensysml.decodeValue(v);
    assert_equal(dv.instanceRef, int64(9), 'instanceRef round trip');

    q = opensysml.encodeValue(struct('magnitude', int64(3), 'unit', 'kg', 'unitTerm', []));
    assert_equal(q.quantity.intMagnitude, '3', 'quantity intMagnitude');
    dq = opensysml.decodeValue(q);
    assert_equal(dq.unit, 'kg', 'quantity round trip');

    e = opensysml.encodeValue(struct('literalId', 'A::b', 'enumerationId', 'A', 'name', 'b', 'value', []));
    assert_equal(e.enumLiteral.literalId, 'A::b', 'enumLiteral');
    de = opensysml.decodeValue(e);
    assert_equal(de.name, 'b', 'enumLiteral round trip');

    v = opensysml.encodeValue(struct('infinity', true));
    assert_equal(v.infinity, true, 'infinity');
    di = opensysml.decodeValue(v);
    assert_equal(di.infinity, true, 'infinity round trip');

    f = opensysml.encodeValue(struct('calcId', 'C::f', 'self', []));
    assert_equal(f.function.calcId, 'C::f', 'function');
    assert_error(@() opensysml.encodeValue(struct('calcId', 'C::f', 'self', struct('instanceRef', int64(1)))), 'opensysml:encode', 'function with self');

    s = opensysml.encodeValue(struct('set', {{int64(1), 'x'}}));
    assert_equal(numel(s.set.elements), 2, 'set elements');

    mo = opensysml.encodeValue(struct('elementId', 'e1', 'metaclassId', 'PartDef'));
    assert_equal(mo.metaobject.elementId, 'e1', 'metaobject');

    seq = opensysml.encodeValue({int64(1), 'x'});
    assert_equal(numel(seq.sequence.elements), 2, 'cell sequence');
    assert_equal(seq.sequence.elements{1}.intValue, '1', 'cell sequence.0');
    seq = opensysml.encodeValue(int64([1 2]));
    assert_equal(numel(seq.sequence.elements), 2, 'numeric sequence');
    assert_equal(seq.sequence.elements{2}.intValue, '2', 'numeric sequence.1');
    empty = opensysml.encodeValue({});
    assert_equal(numel(empty.sequence.elements), 0, 'empty list');

    assert_error(@() opensysml.encodeValue(struct('unset', true)), 'opensysml:encode', 'unset cannot be sent');
    assert_error(@() opensysml.encodeValue(struct('reason', 'r', 'lower', 0, 'upper', 1)), 'opensysml:encode', 'undetermined cannot be sent');

    % -2^63 cannot be written as a double literal; check the round trip
    % through the wire spelling
    m = -bitshift(int64(1), 63);
    assert_equal(opensysml.decodeValue(opensysml.encodeValue(m)), m, 'min int64 round trip');
    assert_equal(opensysml.decodeValue(opensysml.encodeValue('abc')), 'abc', 'string round trip');
    fprintf('encode_value ok\n');
end
