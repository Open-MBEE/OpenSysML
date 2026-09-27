function test_decode_value()
%TEST_DECODE_VALUE All nineteen Value arms and the failure modes.

    % the double literal cannot write -2^63; decodeValue must go through
    % the int64 accumulator
    v = opensysml.decodeValue(struct('intValue', '-9223372036854775808'));
    assert_equal(v, min_int64(), 'min int64');
    assert_equal(opensysml.decodeValue(struct('intValue', '7')), int64(7), 'intValue');
    assert_equal(opensysml.decodeValue(struct('realValue', 1.5)), 1.5, 'realValue');
    assert_equal(opensysml.decodeValue(struct('realValue', 'NaN')), NaN, 'realValue NaN');
    assert_equal(opensysml.decodeValue(struct('realValue', 'Infinity')), Inf, 'realValue Infinity');
    assert_equal(opensysml.decodeValue(struct('realValue', '-Infinity')), -Inf, 'realValue -Infinity');
    assert_equal(opensysml.decodeValue(struct('boolValue', true)), true, 'boolValue');
    assert_equal(opensysml.decodeValue(struct('stringValue', 'abc')), 'abc', 'stringValue');

    v = opensysml.decodeValue(struct('instanceId', '5'));
    assert_equal(v.instanceRef, int64(5), 'instanceId');

    v = opensysml.decodeValue(struct('sequence', struct('elements', {{struct('intValue', '1'), struct('stringValue', 'x')}})));
    assert_equal(v{1}, int64(1), 'sequence.0');
    assert_equal(v{2}, 'x', 'sequence.1');

    assert_equal(opensysml.decodeValue(struct('null', '')), [], 'null');
    assert_error(@() opensysml.decodeValue(struct('null', 'x')), 'opensysml:unsupported', 'non-empty null');

    v = opensysml.decodeValue(struct('unset', struct()));
    assert_equal(v.unset, true, 'unset');

    q = opensysml.decodeValue(struct('quantity', struct('intMagnitude', '3', 'unit', 'kg')));
    assert_equal(q.magnitude, int64(3), 'quantity magnitude');
    assert_equal(q.unit, 'kg', 'quantity unit');
    q = opensysml.decodeValue(struct('quantity', struct('realMagnitude', 2.5, 'unit', 'm')));
    assert_equal(q.magnitude, 2.5, 'quantity realMagnitude');

    e = opensysml.decodeValue(struct('enumLiteral', struct('literalId', 'A::b', 'enumerationId', 'A', 'name', 'b')));
    assert_equal(e.literalId, 'A::b', 'enumLiteral literalId');
    e = opensysml.decodeValue(struct('enumLiteral', struct('literalId', 'A::b', 'value', struct('intValue', '2'))));
    assert_equal(e.value, int64(2), 'enumLiteral value');

    c = opensysml.decodeValue(struct('complex', struct('real', 1, 'imaginary', -2)));
    assert_equal(c, complex(1, -2), 'complex');

    a = opensysml.decodeValue(struct('array', struct('dimensions', 2, 'elements', {{struct('intValue', '1'), struct('intValue', '2')}})));
    assert_equal(a.dimensions, int64(2), 'array dimensions');
    assert_equal(numel(a.elements), 2, 'array elements');
    assert_error(@() opensysml.decodeValue(struct('array', struct('dimensions', 3, 'elements', {{struct('intValue', '1')}}))), 'opensysml:decode', 'array size mismatch');

    vec = opensysml.decodeValue(struct('vector', struct('components', {{struct('realValue', 1), struct('intValue', '2')}})));
    assert_equal(vec.components{1}, 1, 'vector.0');
    assert_error(@() opensysml.decodeValue(struct('vector', struct('components', {{struct('stringValue', 'x')}}))), 'opensysml:decode', 'vector non-numeric');

    vq = opensysml.decodeValue(struct('vectorQuantity', struct('components', {{struct('realMagnitude', 1, 'unit', 'm')}})));
    assert_equal(vq.components{1}.unit, 'm', 'vectorQuantity unit');
    assert_error(@() opensysml.decodeValue(struct('vectorQuantity', struct('components', {}))), 'opensysml:decode', 'empty vectorQuantity');

    m = opensysml.decodeValue(struct('measurementRef', struct('unit', 'kg', 'unitId', 'SI::kg', 'unitTerm', 'mass')));
    assert_equal(m.unitId, 'SI::kg', 'measurementRef unitId');

    v = opensysml.decodeValue(struct('infinity', true));
    assert_equal(v.infinity, true, 'infinity');
    assert_error(@() opensysml.decodeValue(struct('infinity', false)), 'opensysml:decode', 'infinity false');

    f = opensysml.decodeValue(struct('function', struct('calcId', 'C::f', 'selfId', '7')));
    assert_equal(f.calcId, 'C::f', 'function calcId');
    assert_equal(f.self.instanceRef, int64(7), 'function self');
    assert_error(@() opensysml.decodeValue(struct('function', struct('calcId', ''))), 'opensysml:decode', 'empty calcId');
    f = opensysml.decodeValue(struct('function', struct('calcId', 'C::f', 'selfId', '0')));
    assert_equal(isempty(f.self), true, 'function self 0 is no self');

    s = opensysml.decodeValue(struct('set', struct('elements', {{struct('intValue', '1'), struct('intValue', '2')}})));
    assert_equal(s.set{2}, int64(2), 'set.1');
    assert_error(@() opensysml.decodeValue(struct('set', struct('elements', {{struct('intValue', '1'), struct('intValue', '1')}}))), 'opensysml:decode', 'set duplicate');

    t = opensysml.decodeValue(struct('tensorQuantity', struct('dimensions', 2, 'components', {{struct('realMagnitude', 1, 'unit', 'm'), struct('realMagnitude', 2, 'unit', 'm')}})));
    assert_equal(numel(t.components), 2, 'tensorQuantity components');
    assert_error(@() opensysml.decodeValue(struct('tensorQuantity', struct('dimensions', 3, 'components', {{struct('realMagnitude', 1)}}))), 'opensysml:decode', 'tensor size mismatch');

    mo = opensysml.decodeValue(struct('metaobject', struct('elementId', 'e1', 'metaclassId', 'PartDef')));
    assert_equal(mo.elementId, 'e1', 'metaobject');
    assert_error(@() opensysml.decodeValue(struct('metaobject', struct('elementId', ''))), 'opensysml:decode', 'empty elementId');

    u = opensysml.decodeValue(struct('undetermined', struct('reason', 'r', 'count', struct('lower', '0', 'upper', '9'))));
    assert_equal(u.reason, 'r', 'undetermined reason');
    assert_equal(u.lower, '0', 'undetermined lower');

    assert_error(@() opensysml.decodeValue(struct('mystery', struct())), 'opensysml:unknownArm', 'unknown arm');
    fprintf('decode_value ok\n');
end

function m = min_int64()
    m = intmin('int64');
end

