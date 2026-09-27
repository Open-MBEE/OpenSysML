function out = decodeValueMap(mapStruct)
%DECODEVALUEMAP A map<string,Value> wire object -> a struct of decoded
%values keyed by name. Every field's body is one Value object.

    out = struct();
    names = fieldnames(mapStruct);
    for i = 1:numel(names)
        out.(names{i}) = opensysml.decodeValue(mapStruct.(names{i}));
    end
end
