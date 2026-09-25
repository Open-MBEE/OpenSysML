function text = json_write(value)
%JSON_WRITE Compact JSON for a json_tree value: containers.Map -> object,
%cell -> list, char -> string, logical -> true/false, double -> number,
%[] -> null. Requests are sent to the service as this text.

    if isa(value, 'containers.Map')
        keys = value.keys;
        parts = {};
        for i = 1:numel(keys)
            parts{end+1} = [json_string(keys{i}) ':' json_write(value(keys{i}))];
        end
        text = ['{' strjoin(parts, ',') '}'];
    elseif iscell(value)
        parts = cellfun(@json_write, value, 'UniformOutput', false);
        text = ['[' strjoin(parts, ',') ']'];
    elseif ischar(value)
        text = json_string(value);
    elseif islogical(value)
        if value, text = 'true'; else, text = 'false'; end
    elseif isnumeric(value)
        if isempty(value)
            text = 'null';
        elseif isscalar(value)
            if isequal(value, floor(value)) && isfinite(value) && abs(value) < 2^53
                text = sprintf('%d', int64(value));
            else
                text = sprintf('%.15g', value);
            end
        else
            parts = {};
            for i = 1:numel(value), parts{end+1} = json_write(value(i)); end
            text = ['[' strjoin(parts, ',') ']'];
        end
    else
        error('opensysml:json', 'cannot write a value of class %s', class(value));
    end
end

function s = json_string(value)
    out = '"';
    for c = value
        switch c
            case '"', out = [out '\"'];
            case '\', out = [out '\\'];
            case char(8), out = [out '\b'];
            case char(12), out = [out '\f'];
            case char(10), out = [out '\n'];
            case char(13), out = [out '\r'];
            case char(9), out = [out '\t'];
            otherwise
                if c < 32
                    out = [out sprintf('\\u%04x', c)];
                else
                    out(end+1) = c;
                end
        end
    end
    s = [out '"'];
end
