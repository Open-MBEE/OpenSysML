function request = runOptions(request, options, label)
%RUNOPTIONS Add the run options of the sysml command to an execution request.
%   advance (simulated seconds to run for), seed (the model's own draws), draws
%   ('random', 'min', 'max' or 'average') and clockStep are sent as
%   ExecuteAction and ExecuteState carry them; an unset one sends nothing.

    if ~isempty(options.advance)
        if ~(isnumeric(options.advance) && isscalar(options.advance) && isfinite(options.advance) ...
                && options.advance >= 0)
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s advance must be a finite non-negative number of seconds', label));
        end
        request.advance = double(options.advance);
    end
    if ~isempty(options.seed)
        request.seed = seedText(options.seed, label);
    end
    if ~isempty(options.draws)
        draws = char(options.draws);
        if ~any(strcmp(draws, {'random', 'min', 'max', 'average'}))
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s draws must be ''random'', ''min'', ''max'' or ''average''', label));
        end
        request.draws = draws;
    end
    if ~(isnumeric(options.clockStep) && isscalar(options.clockStep) && isfinite(options.clockStep) ...
            && options.clockStep >= 0)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s clockStep must be a finite non-negative number of seconds', label));
    end
    if options.clockStep > 0, request.clockStep = double(options.clockStep); end
end

% The seed is a uint64 on the wire, sent as decimal text: a uint64 or non-negative int64
% is kept exactly; a double must be a finite integer no greater than flintmax, since a
% larger one may not be the integer the caller meant.
function text = seedText(value, label)
    if isa(value, 'uint64') && isscalar(value)
        text = sprintf('%u', value);
        return;
    end
    if isa(value, 'int64') && isscalar(value) && value >= 0
        text = sprintf('%u', uint64(value));
        return;
    end
    if ~(isnumeric(value) && ~islogical(value) && isscalar(value) && isfinite(double(value)) && ...
            double(value) >= 0 && double(value) == fix(double(value)))
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s seed must be a non-negative integer no larger than a uint64', label));
    end
    if double(value) > flintmax
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s seed values above flintmax must be supplied as uint64', label));
    end
    text = sprintf('%u', uint64(value));
end
