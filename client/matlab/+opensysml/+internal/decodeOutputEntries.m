function values = decodeOutputEntries(raw, conn, instances)
%DECODEOUTPUTENTRIES Decode named CalcOutput values to a struct.

    if nargin < 3, instances = []; end
    values = struct();
    if isempty(raw), return; end
    raw = toCells(raw);
    for i = 1:numel(raw)
        item = raw{i};
        name = item.name;
        try
            if isa(instances, 'containers.Map')
                value = opensysml.decodeValue(item.value, ...
                    opensysml.internal.instanceResolver(instances));
            else
                value = opensysml.decodeValue(item.value);
            end
        catch e
            value = e;
        end
        values.(name) = value;
    end
end

function values = toCells(raw)
    if iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {};
    end
end
