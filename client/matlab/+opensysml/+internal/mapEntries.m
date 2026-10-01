function [names, values] = mapEntries(raw)
%MAPENTRIES Return names and values from a decoded JSON map.

    if isa(raw, 'containers.Map')
        names = raw.keys;
        values = cell(1, numel(names));
        for i = 1:numel(names), values{i} = raw(names{i}); end
    elseif isstruct(raw)
        names = fieldnames(raw)';
        values = cell(1, numel(names));
        for i = 1:numel(names)
            values{i} = raw.(names{i});
        end
    elseif isempty(raw)
        names = {};
        values = {};
    else
        opensysml.internal.raise('opensysml:argument', ...
            'map entries must be a struct, containers.Map, or empty');
    end
end
