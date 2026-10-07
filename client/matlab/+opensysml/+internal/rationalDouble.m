function x = rationalDouble(terms)
%RATIONALDOUBLE The double a Rational's decimal terms are exactly; empty when none is.
%   Read where both terms are exact doubles; larger terms are taken as no double.

    x = [];
    n = str2double(terms.numerator);
    d = str2double(terms.denominator);
    if abs(n) <= flintmax && d <= flintmax && bitand(uint64(d), uint64(d) - 1) == 0
        x = n / d;
    end
end
