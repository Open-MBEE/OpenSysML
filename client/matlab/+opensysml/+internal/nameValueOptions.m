function options = nameValueOptions(defaults, args, label)
%NAMEVALUEOPTIONS Parse a package function's name-value arguments.

    options = defaults;
    if mod(numel(args), 2) ~= 0
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s options must be name-value pairs', label));
    end
    names = fieldnames(defaults);
    for i = 1:2:numel(args)
        key = args{i};
        if isstringValue(key), key = char(key); end
        if ~ischar(key)
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s option names must be character vectors', label));
        end
        if ~any(strcmp(names, key))
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('%s has no option ''%s''', label, key));
        end
        options.(key) = args{i+1};
    end
end

function tf = isstringValue(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value) && isscalar(value);
    end
end
