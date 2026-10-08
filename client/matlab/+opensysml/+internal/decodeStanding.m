function standing = decodeStanding(raw)
%DECODESTANDING Decode an engine standing from result fields.

    bounds = toCells(fieldOr(raw, 'bounds', {}));
    for i = 1:numel(bounds)
        if isfield(bounds{i}, 'limit')
            limit = bounds{i}.limit;
            if ischar(limit), bounds{i}.limit = opensysml.parseInt64(limit);
            else, bounds{i}.limit = int64(limit);
            end
        end
    end
    standing = opensysml.Standing(fieldOr(raw, 'engine', ''), ...
        fieldOr(raw, 'strength', ''), bounds);
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {};
    end
end
