function value = parseUint64(raw)
%PARSEUINT64 Parse a decimal wire integer without passing through double.

    if isa(raw, 'uint64'), value = raw; return; end
    if isa(raw, 'int64')
        if raw < 0
            opensysml.internal.raise('opensysml:decode', 'uint64 value is negative');
        end
        value = uint64(raw);
        return;
    end
    if isnumeric(raw) && isscalar(raw) && isfinite(double(raw)) && ...
            double(raw) >= 0 && double(raw) == fix(double(raw)) && ...
            double(raw) <= flintmax
        value = uint64(raw);
        return;
    end
    if isStringValue(raw), raw = char(raw); end
    if ~ischar(raw) || isempty(raw) || any(raw < '0' | raw > '9')
        opensysml.internal.raise('opensysml:decode', 'invalid uint64 value');
    end
    value = uint64(0);
    limit = intmax('uint64');
    for i = 1:numel(raw)
        digit = uint64(double(raw(i)) - double('0'));
        if value > idivide(limit - digit, uint64(10), 'floor')
            opensysml.internal.raise('opensysml:decode', 'uint64 value is out of range');
        end
        value = value * uint64(10) + digit;
    end
end

function tf = isStringValue(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value) && isscalar(value);
    end
end
