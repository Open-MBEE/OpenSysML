function result = verifyConstraint(model, symbolId, varargin)
%VERIFYCONSTRAINT Ask whether a constraint holds.

    result = opensysml.internal.verifyOne(model, symbolId, 'VerifyConstraint', ...
        'Constraint', varargin{:});
end
