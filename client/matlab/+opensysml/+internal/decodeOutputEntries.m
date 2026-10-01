function values = decodeOutputEntries(raw, conn, instances)
%DECODEOUTPUTENTRIES Decode named calculation values.

    if nargin < 3, instances = []; end
    entries = outputEntries(raw);
    if ~isempty(entries)
        names = cellfun(@(item) item.name, entries, 'UniformOutput', false);
        useMap = any(~cellfun(@isvarname, names));
        if useMap, values = containers.Map('KeyType', 'char', 'ValueType', 'any');
        else, values = struct();
        end
        for i = 1:numel(entries)
            item = entries{i};
            name = item.name;
            value = decodeOne(item.value, instances);
            values = setOutput(values, useMap, name, value);
        end
        return;
    end
    [names, wireValues] = opensysml.internal.mapEntries(raw);
    useMap = isa(raw, 'containers.Map') || any(~cellfun(@isvarname, names));
    if useMap, values = containers.Map('KeyType', 'char', 'ValueType', 'any');
    else, values = struct();
    end
    for i = 1:numel(names)
        value = decodeOne(wireValues{i}, instances);
        values = setOutput(values, useMap, names{i}, value);
    end
end

function entries = outputEntries(raw)
    entries = {};
    if iscell(raw)
        items = raw(:)';
        if ~isempty(items) && isstruct(items{1}) && ...
                isfield(items{1}, 'name') && isfield(items{1}, 'value') && ...
                ischar(items{1}.name)
            entries = items;
        end
    elseif isstruct(raw) && ~isempty(raw) && ...
            isfield(raw, 'name') && isfield(raw, 'value') && ischar(raw(1).name)
        entries = num2cell(raw(:)');
    end
end

function values = setOutput(values, useMap, name, value)
    if useMap
        values(name) = value;
    else
        values.(name) = value;
    end
end

function value = decodeOne(raw, instances)
    try
        if isa(instances, 'containers.Map')
            value = opensysml.decodeValue(raw, ...
                opensysml.internal.instanceResolver(instances));
        else
            value = opensysml.decodeValue(raw);
        end
    catch e
        value = e;
    end
end
