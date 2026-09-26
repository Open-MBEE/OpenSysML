function [value, found] = conf_lookup(value, path)
%CONF_LOOKUP Walk a dotted path over json_tree values; expected keys are
%tried exactly then lowerCamel — actual keys are never renamed.

    [value, found] = lookup_parts(value, strsplit(path, '.'));
end

function out = lower_camel(k)
    parts = strsplit(k, '_');
    out = parts{1};
    for i = 2:numel(parts)
        p = parts{i};
        if ~isempty(p), out = [out upper(p(1)) p(2:end)]; end
    end
end

function [value, found] = getMember(value, key)
    found = false;
    if isa(value, 'containers.Map')
        if isKey(value, key)
            value = value(key); found = true; return;
        end
        camel = lower_camel(key);
        if ~strcmp(camel, key) && isKey(value, camel)
            value = value(camel); found = true; return;
        end
        value = []; return;
    elseif iscell(value)
        i = str2double(key);
        if ~isnan(i) && i == floor(i) && i >= 0 && i < numel(value)
            value = value{i+1}; found = true; return;
        end
        value = []; return;
    end
    value = [];
end

function [value, found] = lookup_parts(value, parts)
    if isempty(parts)
        found = true; return;
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
            [got, ok] = lookup_parts(items{i}, rest);
            if ok
                value = got; found = true; return;
            end
        end
        value = []; found = false; return;
    end
    [next, found] = getMember(value, head);
    if ~found
        value = []; found = false; return;
    end
    [value, found] = lookup_parts(next, rest);
end
