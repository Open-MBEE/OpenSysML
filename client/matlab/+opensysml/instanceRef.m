function value = instanceRef(id)
%INSTANCEREF Construct an instance reference value.

    if isa(id, 'int64') && isscalar(id)
        parsed = id;
    elseif ischar(id)
        parsed = opensysml.parseInt64(id);
    elseif isinteger(id) && isscalar(id) && ~islogical(id)
        if isa(id, 'uint64') && id > uint64(intmax('int64'))
            opensysml.internal.raise('opensysml:argument', ...
                'instance reference id is outside the signed 64-bit range');
        end
        parsed = int64(id);
    elseif isnumeric(id) && isscalar(id) && isfinite(id) && fix(id) == id
        if id < -9223372036854775808 || id >= 9223372036854775808
            opensysml.internal.raise('opensysml:argument', ...
                'instance reference id is outside the signed 64-bit range');
        end
        parsed = int64(id);
    else
        opensysml.internal.raise('opensysml:argument', ...
            'instance reference id must be an integer scalar');
    end
    value = struct('instanceRef', parsed);
end
