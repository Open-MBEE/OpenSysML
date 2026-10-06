function binding = bindingRationalsAsReals(binding)
%BINDINGRATIONALSASREALS A wire binding with each Rational a double holds sent as that double.
%   The form a service without rational_values reads; other Rationals are kept.

    for i = 1:numel(binding.values)
        value = binding.values{i};
        if isfield(value, 'rationalValue')
            exact = opensysml.internal.rationalDouble(value.rationalValue);
            if ~isempty(exact), binding.values{i} = struct('realValue', exact); end
        elseif isfield(value, 'quantity') && isfield(value.quantity, 'rationalMagnitude')
            exact = opensysml.internal.rationalDouble(value.quantity.rationalMagnitude);
            if ~isempty(exact)
                quantity = rmfield(value.quantity, 'rationalMagnitude');
                quantity.realMagnitude = exact;
                binding.values{i}.quantity = quantity;
            end
        end
    end
end
