function value = decodeJson(text)
%DECODEJSON Decode JSON while preserving object keys MATLAB cannot name.

    quotePositions = strfind(text, '"');
    originals = {};
    editStarts = [];
    editEnds = [];
    editText = {};
    last = 1;
    quoteIndex = 1;
    while quoteIndex <= numel(quotePositions)
        start = quotePositions(quoteIndex);
        closeIndex = quoteIndex + 1;
        while closeIndex <= numel(quotePositions)
            finish = quotePositions(closeIndex);
            slashCount = 0;
            previous = finish - 1;
            while previous >= start + 1 && text(previous) == '\'
                slashCount = slashCount + 1;
                previous = previous - 1;
            end
            if mod(slashCount, 2) == 0, break; end
            closeIndex = closeIndex + 1;
        end
        if closeIndex > numel(quotePositions), break; end
        token = text(start:finish);
        prefix = text(last:start-1);
        rewrittenPrefix = rewriteNegativeZero(prefix);
        if ~strcmp(rewrittenPrefix, prefix)
            editStarts(end+1) = last;
            editEnds(end+1) = start - 1;
            editText{end+1} = rewrittenPrefix;
        end
        next = finish + 1;
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
                editStarts(end+1) = start;
                editEnds(end+1) = finish;
                editText{end+1} = sprintf('"osk_x_%d"', index);
            end
        end
        last = finish + 1;
        quoteIndex = closeIndex + 1;
    end
    suffix = text(last:end);
    rewrittenSuffix = rewriteNegativeZero(suffix);
    if ~strcmp(rewrittenSuffix, suffix)
        editStarts(end+1) = last;
        editEnds(end+1) = numel(text);
        editText{end+1} = rewrittenSuffix;
    end
    if isempty(editStarts)
        value = jsondecode(text);
    else
        parts = {};
        cursor = 1;
        for i = 1:numel(editStarts)
            if editStarts(i) > cursor
                parts{end+1} = text(cursor:editStarts(i)-1);
            end
            parts{end+1} = editText{i};
            cursor = editEnds(i) + 1;
        end
        if cursor <= numel(text), parts{end+1} = text(cursor:end); end
        value = jsondecode([parts{:}]);
    end
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
