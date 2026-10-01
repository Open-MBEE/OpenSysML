function value = decodeJson(text)
%DECODEJSON Decode JSON while preserving object keys MATLAB cannot name.

    quotePositions = strfind(text, '"');
    escapedQuotes = false(size(quotePositions));
    backslashQuotes = find(quotePositions > 1);
    backslashQuotes = backslashQuotes( ...
        text(quotePositions(backslashQuotes) - 1) == '\');
    for i = 1:numel(backslashQuotes)
        quoteIndex = backslashQuotes(i);
        position = quotePositions(quoteIndex) - 1;
        count = 0;
        while position > 0 && text(position) == '\'
            count = count + 1;
            position = position - 1;
        end
        escapedQuotes(quoteIndex) = mod(count, 2) == 1;
    end
    unescapedQuotes = quotePositions(~escapedQuotes);
    quoteOpens = unescapedQuotes(1:2:end);
    quoteCloses = unescapedQuotes(2:2:end);
    pairCount = numel(quoteCloses);
    quoteOpens = quoteOpens(1:pairCount);

    keyCloseCandidates = strfind(text, '":');
    isKeyClose = ismember(quoteCloses, keyCloseCandidates);
    spacedKeyCloseCandidates = quoteCloses(quoteCloses < numel(text));
    spacedKeyCloseCandidates = spacedKeyCloseCandidates( ...
        isspace(text(spacedKeyCloseCandidates + 1)));
    if ~isempty(spacedKeyCloseCandidates)
        colonPositions = strfind(text, ':');
        nonWhitespacePositions = find(~isspace(text));
        [~, colonBuckets] = histc(colonPositions, [nonWhitespacePositions Inf]);
        hasPrevious = colonBuckets > 1;
        previousNonWhitespace = nonWhitespacePositions(colonBuckets(hasPrevious) - 1);
        isSpacedKeyClose = ismember(spacedKeyCloseCandidates, previousNonWhitespace);
        isKeyClose = isKeyClose | ...
            ismember(quoteCloses, spacedKeyCloseCandidates(isSpacedKeyClose));
    end
    keyOpens = quoteOpens(isKeyClose);
    keyCloses = quoteCloses(isKeyClose);
    keyLengths = keyCloses - keyOpens - 1;
    keywordNames = {'break', 'case', 'catch', 'classdef', 'continue', ...
        'else', 'elseif', 'end', 'for', 'function', 'global', 'if', 'otherwise', ...
        'parfor', 'persistent', 'return', 'spmd', 'switch', 'try', 'while'};
    keywordPattern = ['"(break|case|catch|classdef|continue|else|elseif|' ...
        'end|for|function|global|if|otherwise|parfor|persistent|return|spmd|' ...
        'switch|try|while|osk_x_\w*)"\s*:'];
    needsCheck = keyLengths > namelengthmax | keyLengths == 0;
    if ~isempty(keyOpens)
        firstChars = text(keyOpens + 1);
        firstLetters = (firstChars >= 'A' & firstChars <= 'Z') | ...
            (firstChars >= 'a' & firstChars <= 'z');
        needsCheck = needsCheck | ~firstLetters;
        keyEvents = zeros(1, numel(text) + 1, 'int8');
        keyStarts = keyOpens + 1;
        keyEvents(keyStarts) = keyEvents(keyStarts) + 1;
        keyEvents(keyCloses) = keyEvents(keyCloses) - 1;
        keyCharacterMask = cumsum(keyEvents(1:numel(text))) > 0;
        keyCharacterPositions = find(keyCharacterMask);
        keyCharacters = text(keyCharacterPositions);
        invalidKeyOffsets = regexp(keyCharacters, '[^A-Za-z0-9_"]', 'start');
        if ~isempty(invalidKeyOffsets)
            nonWordPositions = keyCharacterPositions(invalidKeyOffsets);
            [~, buckets] = histc(nonWordPositions, [keyOpens Inf]);
            validBuckets = buckets > 0 & buckets <= numel(keyOpens);
            insideKeys = false(size(buckets));
            insideKeys(validBuckets) = ...
                nonWordPositions(validBuckets) < keyCloses(buckets(validBuckets));
            needsCheck(unique(buckets(insideKeys))) = true;
        end
        tokenLengths = keyLengths + 3;
        tokenStarts = [1 1 + cumsum(tokenLengths(1:end-1))];
        keyTokens = repmat(' ', 1, sum(tokenLengths));
        keyTokens(tokenStarts) = '"';
        keyTokens(tokenStarts + keyLengths + 1) = '"';
        keyTokens(tokenStarts + keyLengths + 2) = ':';
        if any(keyLengths)
            charIndices = 1:sum(keyLengths);
            precedingLengths = [0 cumsum(keyLengths(1:end-1))];
            keyIndices = repelem(1:numel(keyLengths), keyLengths);
            charOffsets = charIndices - repelem(precedingLengths, keyLengths);
            tokenPositions = tokenStarts(keyIndices) + charOffsets;
            keyTokens(tokenPositions) = keyCharacters;
        end
        keywordStarts = regexp(keyTokens, keywordPattern, 'start');
        needsCheck = needsCheck | ismember(tokenStarts, keywordStarts);
    end

    originals = {};
    editStarts = [];
    editEnds = [];
    editText = {};
    checkedKeys = find(needsCheck);
    for i = 1:numel(checkedKeys)
        keyIndex = checkedKeys(i);
        key = jsondecode(text(keyOpens(keyIndex):keyCloses(keyIndex)));
        if needsKeyEscape(key, keywordNames)
            originalIndex = find(strcmp(originals, key), 1);
            if isempty(originalIndex)
                originals{end+1} = key;
                originalIndex = numel(originals);
            end
            editStarts(end+1) = keyOpens(keyIndex);
            editEnds(end+1) = keyCloses(keyIndex);
            editText{end+1} = sprintf('"osk_x_%d"', originalIndex);
        end
    end

    zeroStarts = [];
    zeroEnds = [];
    zeroMatches = {};
    if ~isempty(strfind(text, '-0'))
        [zeroStarts, zeroEnds, zeroMatches] = regexp(text, ...
            '(^|[:,\[\{\s])-0(?![.eE0-9])', 'start', 'end', 'match');
    end
    if ~isempty(zeroStarts)
        tokenStarts = zeroEnds - 1;
        if isempty(unescapedQuotes)
            outsideStrings = true(size(zeroStarts));
        else
            [~, quoteCounts] = histc(tokenStarts, [-Inf unescapedQuotes Inf]);
            outsideStrings = mod(quoteCounts - 1, 2) == 0;
        end
        zeroIndices = find(outsideStrings);
        for i = 1:numel(zeroIndices)
            zeroIndex = zeroIndices(i);
            matched = zeroMatches{zeroIndex};
            editStarts(end+1) = zeroStarts(zeroIndex);
            editEnds(end+1) = zeroEnds(zeroIndex);
            editText{end+1} = [matched(1:end-2) '-0.0'];
        end
    end

    if isempty(editStarts)
        value = jsondecode(text);
    else
        [editStarts, order] = sort(editStarts);
        editEnds = editEnds(order);
        editText = editText(order);
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

function tf = needsKeyEscape(key, keywords)
    if isempty(key)
        tf = true;
        return;
    end
    first = key(1);
    firstLetter = (first >= 'A' && first <= 'Z') || ...
        (first >= 'a' && first <= 'z');
    rest = key(2:end);
    restValid = all((rest >= 'A' & rest <= 'Z') | ...
        (rest >= 'a' & rest <= 'z') | ...
        (rest >= '0' & rest <= '9') | rest == '_');
    tf = ~firstLetter || ~restValid || length(key) > namelengthmax || ...
        ~isvarname(key) || any(strcmp(key, keywords)) || ...
        strncmp(key, 'osk_x_', 6);
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
