function n = parseInt64(s)
%PARSEINT64 Exact int64 from a decimal string, without a double anywhere
%on the way. Digits accumulate in negative space so "-9223372036854775808" stays exact.

    if isnumeric(s)
        n = int64(s);
        return;
    end
    s = char(s);
    neg = false;
    if ~isempty(s) && s(1) == '-'
        neg = true;
        s = s(2:end);
    elseif ~isempty(s) && s(1) == '+'
        s = s(2:end);
    end
    acc = int64(0);
    for c = s
        if c < '0' || c > '9'
            error('opensysml:transport', 'not an int64 literal: %s', s);
        end
        acc = acc * int64(10) - int64(c - '0');
    end
    if neg
        n = acc;
    else
        n = -acc;
    end
end
