function values = decodeNamedValues(raw, conn, asMap)
%DECODENAMEDVALUES Decode a map of wire Values without dropping bad arms.

    if nargin < 3, asMap = false; end
    [names, rawValues] = opensysml.internal.mapEntries(raw);
    useMap = asMap || isa(raw, 'containers.Map') || ...
        any(~cellfun(@isvarname, names));
    if useMap
        values = containers.Map('KeyType', 'char', 'ValueType', 'any');
    else
        values = struct();
    end
    for i = 1:numel(names)
        try
            value = opensysml.decodeValue(rawValues{i});
        catch e
            value = e;
        end
        if useMap
            values(names{i}) = value;
        else
            values.(names{i}) = value;
        end
    end
end
