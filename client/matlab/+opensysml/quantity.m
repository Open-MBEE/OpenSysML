function value = quantity(magnitude, unit, unitTerm)
%QUANTITY Construct a quantity value.

    if nargin < 2, unit = ''; end
    if nargin < 3, unitTerm = []; end
    if ~isnumeric(magnitude) || ~isscalar(magnitude) || ~isreal(magnitude)
        opensysml.internal.raise('opensysml:argument', ...
            'quantity magnitude must be a real numeric scalar');
    end
    value = struct('magnitude', magnitude, 'unit', char(unit), 'unitTerm', unitTerm);
end
