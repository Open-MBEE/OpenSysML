function values = conf_values_at(value, path)
%CONF_VALUES_AT All values the dotted path collects across `*` wildcards.

    values = values_parts(value, strsplit(path, '.'));
end

function values = values_parts(value, parts)
    values = {};
    if isempty(parts)
        values = {value}; return;
    end
    head = parts{1}; rest = parts(2:end);
    if strcmp(head, '*')
        items = {};
        if iscell(value)
            items = value;
        elseif isa(value, 'containers.Map')
            items = value.values;
        end
        for i = 1:numel(items)
            values = [values values_parts(items{i}, rest)];
        end
        return;
    end
    next = []; found = false;
    if isa(value, 'containers.Map')
        if isKey(value, head)
            next = value(head); found = true;
        else
            camel = conf_lower_camel(head);
            if ~strcmp(camel, head) && isKey(value, camel)
                next = value(camel); found = true;
            end
        end
    elseif iscell(value)
        i = str2double(head);
        if ~isnan(i) && i == floor(i) && i >= 0 && i < numel(value)
            next = value{i+1}; found = true;
        end
    end
    if found
        values = values_parts(next, rest);
    end
end
