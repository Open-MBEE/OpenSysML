function r = rationalOf(raw)
%RATIONALOF The exact Rational a rationalValue spells, kept as its decimal terms.
%   Terms are canonical: lowest terms over a positive denominator, of a value no
%   double holds. Lowest terms and double exactness are checked where both terms
%   are exact doubles; larger terms are taken as the service sent them.

    if ~isstruct(raw) || ~isscalar(raw) || ~isfield(raw, 'numerator') || ~isfield(raw, 'denominator')
        error('opensysml:decode', 'a rational carries a numerator and a denominator');
    end
    n = char(raw.numerator);
    d = char(raw.denominator);
    if isempty(regexp(n, '^(0|-?[1-9][0-9]*)$', 'once')) || isempty(regexp(d, '^[1-9][0-9]*$', 'once'))
        error('opensysml:decode', 'not the decimal terms of a rational: %s/%s', n, d);
    end
    nd = str2double(n);
    dd = str2double(d);
    if abs(nd) <= flintmax && dd <= flintmax
        if gcd(nd, dd) ~= 1
            error('opensysml:decode', '%s/%s is not in lowest terms', n, d);
        end
        if bitand(uint64(dd), uint64(dd) - 1) == 0
            error('opensysml:decode', '%s/%s is a double, which realValue carries', n, d);
        end
    end
    r = struct('numerator', n, 'denominator', d);
end
