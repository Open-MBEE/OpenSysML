function values = decodeNamedValues(raw, conn, asMap)
%DECODENAMEDVALUES Decode a map of wire Values without dropping bad arms.

    if nargin < 3, asMap = false; end
    if asMap
        values = containers.Map('KeyType', 'char', 'ValueType', 'any');
    else
        values = struct();
    end
    if isempty(raw), return; end
    names = fieldnames(raw);
    for i = 1:numel(names)
        try
            value = opensysml.decodeValue(raw.(names{i}));
        catch e
            value = e;
        end
        if asMap
            values(names{i}) = value;
        else
            values.(names{i}) = value;
        end
    end
end
