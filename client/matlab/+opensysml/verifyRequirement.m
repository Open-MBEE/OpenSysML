function result = verifyRequirement(model, symbolId, varargin)
%VERIFYREQUIREMENT Ask whether a requirement is satisfied.

    result = opensysml.internal.verifyOne(model, symbolId, 'VerifyRequirement', ...
        'Requirement', varargin{:});
end
