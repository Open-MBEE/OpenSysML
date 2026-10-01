function value = decodeJson(text)
%DECODEJSON Decode JSON while preserving object keys MATLAB cannot name.

    [starts, ends, matches] = regexp(text, '"(?:[^"\\]|\\.)*"', ...
        'start', 'end', 'match');
    originals = {};
    parts = {};
    last = 1;
    for i = 1:numel(matches)
        parts{end+1} = rewriteNegativeZero(text(last:starts(i)-1));
        token = matches{i};
        next = ends(i) + 1;
        while next <= numel(text) && isspace(text(next)), next = next + 1; end
        if next <= numel(text) && text(next) == ':'
            key = jsondecode(token);
            if ~strcmp(key, 'function') && ...
                    (~isvarname(key) || length(key) > namelengthmax || ...
                    strncmp(key, 'osk_x_', 6))
                index = find(strcmp(originals, key), 1);
                if isempty(index)
                    originals{end+1} = key;
                    index = numel(originals);
                end
                token = sprintf('"osk_x_%d"', index);
            end
        end
        parts{end+1} = token;
        last = ends(i) + 1;
    end
    parts{end+1} = rewriteNegativeZero(text(last:end));
    rewritten = [parts{:}];
    value = jsondecode(rewritten);
    if ~isempty(originals)
        value = restoreKeys(value, originals);
    end
end

function text = rewriteNegativeZero(text)
    text = regexprep(text, '(^|[:,\[\{\s])-0(?![.eE0-9])', '$1-0.0');
end

function value = restoreKeys(value, originals)
    if isstruct(value)
        names = fieldnames(value);
        escaped = any(cellfun(@(name) strncmp(name, 'osk_x_', 6), names));
        if escaped && numel(value) > 1
            restored = cell(size(value));
            for i = 1:numel(value)
                restored{i} = restoreMap(value(i), originals);
            end
            value = restored;
        elseif escaped
            value = restoreMap(value, originals);
        else
            for i = 1:numel(value)
                for j = 1:numel(names)
                    value(i).(names{j}) = restoreKeys(value(i).(names{j}), originals);
                end
            end
        end
    elseif iscell(value)
        for i = 1:numel(value)
            value{i} = restoreKeys(value{i}, originals);
        end
    end
end

function value = restoreMap(raw, originals)
    value = containers.Map('KeyType', 'char', 'ValueType', 'any');
    names = fieldnames(raw);
    for i = 1:numel(names)
        name = names{i};
        if strncmp(name, 'osk_x_', 6)
            index = str2double(name(7:end));
            name = originals{index};
        end
        value(name) = restoreKeys(raw.(names{i}), originals);
    end
end
