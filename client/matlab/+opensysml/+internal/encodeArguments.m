function values = encodeArguments(arguments, conn, label)
%ENCODEARGUMENTS Encode positional values as repeated wire Values.

    if nargin < 3, label = 'arguments'; end
    if isempty(arguments), values = {}; return; end
    if ~iscell(arguments)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be a cell array', label));
    end
    values = cell(1, numel(arguments));
    for i = 1:numel(arguments)
        values{i} = opensysml.encodeValue(arguments{i}, conn);
    end
end
