function value = functionRef(calcId)
%FUNCTIONREF Construct a function value without an object binding.

    value = struct('calcId', char(calcId), 'self', []);
end
