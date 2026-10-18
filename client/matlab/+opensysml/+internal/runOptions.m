function request = runOptions(request, options, label)
%RUNOPTIONS Add the run options of the sysml command to an execution request.
%   advance (simulated seconds to run for), seed (the model's own draws), draws
%   ('random', 'min', 'max' or 'average') and clockStep are sent as
%   ExecuteAction and ExecuteState carry them; an unset one sends nothing.

    if ~isempty(options.advance)
        if ~(isnumeric(options.advance) && isscalar(options.advance) && options.advance >= 0)
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s advance must be a non-negative number of seconds', label));
        end
        request.advance = double(options.advance);
    end
    if ~isempty(options.seed)
        if ~(isnumeric(options.seed) && isscalar(options.seed) && options.seed >= 0 ...
                && options.seed == floor(options.seed))
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s seed must be a non-negative integer', label));
        end
        request.seed = sprintf('%d', uint64(options.seed));
    end
    if ~isempty(options.draws)
        draws = char(options.draws);
        if ~any(strcmp(draws, {'random', 'min', 'max', 'average'}))
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s draws must be ''random'', ''min'', ''max'' or ''average''', label));
        end
        request.draws = draws;
    end
    if ~(isnumeric(options.clockStep) && isscalar(options.clockStep) && options.clockStep >= 0)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s clockStep must be a non-negative number of seconds', label));
    end
    if options.clockStep > 0, request.clockStep = double(options.clockStep); end
end
