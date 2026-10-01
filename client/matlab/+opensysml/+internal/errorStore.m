function value = errorStore(action, entry)
%ERRORSTORE Keep the most recently raised client error.

    persistent stored
    if isempty(stored)
        stored = struct('identifier', '', 'message', '', 'diagnostics', {{}}, 'details', struct());
    end
    if strcmp(action, 'set')
        stored = entry;
    end
    value = stored;
end
