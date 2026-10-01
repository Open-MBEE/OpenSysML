function value = enumLiteral(literalId, enumerationId, name, literalValue)
%ENUMLITERAL Construct an enumeration literal value.

    if nargin < 2, enumerationId = ''; end
    if nargin < 3, name = ''; end
    if nargin < 4, literalValue = []; end
    value = struct('literalId', char(literalId), ...
        'enumerationId', char(enumerationId), 'name', char(name), 'value', literalValue);
end
