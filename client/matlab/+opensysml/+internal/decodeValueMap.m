function out = decodeValueMap(mapStruct)
%DECODEVALUEMAP A map<string,Value> wire object -> a struct of decoded
%values keyed by name. Every field's body is one Value object.

    [names, rawValues] = opensysml.internal.mapEntries(mapStruct);
    useMap = isa(mapStruct, 'containers.Map') || any(~cellfun(@isvarname, names));
    if useMap
        out = containers.Map('KeyType', 'char', 'ValueType', 'any');
    else
        out = struct();
    end
    for i = 1:numel(names)
        try
            value = opensysml.decodeValue(rawValues{i});
        catch e
            value = e;
        end
        if useMap
            out(names{i}) = value;
        else
            out.(names{i}) = value;
        end
    end
end
