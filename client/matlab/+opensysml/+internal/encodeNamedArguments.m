function values = encodeNamedArguments(arguments, conn, label)
%ENCODENAMEDARGUMENTS Encode named values from a struct or containers.Map.

    if nargin < 3, label = 'named arguments'; end
    values = struct();
    if isempty(arguments), return; end
    if isa(arguments, 'containers.Map')
        names = arguments.keys;
        for i = 1:numel(names)
            values.(names{i}) = opensysml.encodeValue(arguments(names{i}), conn);
        end
    elseif isstruct(arguments) && isscalar(arguments)
        names = fieldnames(arguments);
        for i = 1:numel(names)
            values.(names{i}) = opensysml.encodeValue(arguments.(names{i}), conn);
        end
    else
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a scalar struct or containers.Map', label));
    end
end
