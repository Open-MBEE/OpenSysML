function out = decodeValues(x)
%DECODEVALUES Decode every Value object nested inside a decoded response
%tree; a struct with exactly one field naming a known arm is a Value.

    if isstruct(x) && numel(x) == 1
        f = fieldnames(x);
        if numel(f) == 1 && ismember(f{1}, opensysml.internal.valueArms())
            out = opensysml.decodeValue(x);
            return;
        end
        out = x;
        for i = 1:numel(f)
            out.(f{i}) = opensysml.internal.decodeValues(out.(f{i}));
        end
    elseif isstruct(x)
        out = x;
        for i = 1:numel(x)
            out(i) = opensysml.internal.decodeValues(x(i));
        end
    elseif iscell(x)
        out = cellfun(@opensysml.internal.decodeValues, x, 'UniformOutput', false);
    elseif isa(x, 'containers.Map')
        names = x.keys;
        if numel(names) == 1 && strcmp(names{1}, 'function')
            out = opensysml.decodeValue(x);
        else
            out = x;
            for i = 1:numel(names)
                out(names{i}) = opensysml.internal.decodeValues(x(names{i}));
            end
        end
    else
        out = x;
    end
end
