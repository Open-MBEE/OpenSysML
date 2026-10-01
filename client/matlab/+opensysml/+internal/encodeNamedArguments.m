function values = encodeNamedArguments(arguments, conn, label)
%ENCODENAMEDARGUMENTS Encode named values from a struct or containers.Map.

    if nargin < 3, label = 'named arguments'; end
    if isempty(arguments), values = struct(); return; end
    if isa(arguments, 'containers.Map') || (isstruct(arguments) && isscalar(arguments))
        [names, rawValues] = opensysml.internal.mapEntries(arguments);
    else
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a scalar struct or containers.Map', label));
    end
    useMap = any(~cellfun(@isvarname, names) | ...
        cellfun(@(name) length(name) > namelengthmax, names));
    if useMap, values = containers.Map('KeyType', 'char', 'ValueType', 'any');
    else, values = struct();
    end
    for i = 1:numel(names)
        value = opensysml.encodeValue(rawValues{i}, conn);
        if useMap, values(names{i}) = value;
        else, values.(names{i}) = value;
        end
    end
end
