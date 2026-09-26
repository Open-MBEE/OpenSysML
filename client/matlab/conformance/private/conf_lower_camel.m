function out = conf_lower_camel(k)
%CONF_LOWER_CAMEL proto snake_case -> lowerCamelCase wire name.

    parts = strsplit(k, '_');
    out = parts{1};
    for i = 2:numel(parts)
        p = parts{i};
        if ~isempty(p), out = [out upper(p(1)) p(2:end)]; end
    end
end
