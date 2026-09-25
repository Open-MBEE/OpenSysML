function [value, next] = json_tree(text)
%JSON_TREE A faithful JSON reader for the conformance runner.
%   jsondecode loses the distinctions the comparison rules live on — a
%   one-object list vs an object, null vs an empty list, an int64 string vs
%   a number — so the runner reads scenarios and answers with this instead.
%   Objects become containers.Map, lists become cell arrays, strings char,
%   numbers double, booleans logical, null [].

    [value, next] = readValue(text, skipWs(text, 1));
end

function i = skipWs(text, i)
    while i <= numel(text) && ismember(text(i), sprintf(' \t\r\n'))
        i = i + 1;
    end
end

function [value, i] = readValue(text, i)
    i = skipWs(text, i);
    if i > numel(text)
        error('opensysml:json', 'unexpected end of input');
    end
    switch text(i)
        case '{'
            [value, i] = readObject(text, i + 1);
        case '['
            [value, i] = readArray(text, i + 1);
        case '"'
            [value, i] = readString(text, i);
        case {'t', 'f'}
            if strncmp(text(i:end), 'true', 4)
                value = true; i = i + 4;
            elseif strncmp(text(i:end), 'false', 5)
                value = false; i = i + 5;
            else
                error('opensysml:json', 'bad literal at %d', i);
            end
        case 'n'
            if ~strncmp(text(i:end), 'null', 4)
                error('opensysml:json', 'bad literal at %d', i);
            end
            value = []; i = i + 4;
        otherwise
            [value, i] = readNumber(text, i);
    end
end

function [map, i] = readObject(text, i)
    map = containers.Map('KeyType', 'char', 'ValueType', 'any');
    i = skipWs(text, i);
    if i <= numel(text) && text(i) == '}'
        i = i + 1;
        return;
    end
    while true
        i = skipWs(text, i);
        if i > numel(text) || text(i) ~= '"'
            error('opensysml:json', 'expected a key at %d', i);
        end
        [key, i] = readString(text, i);
        i = skipWs(text, i);
        if i > numel(text) || text(i) ~= ':'
            error('opensysml:json', 'expected : at %d', i);
        end
        [child, i] = readValue(text, i + 1);
        map(key) = child;
        i = skipWs(text, i);
        if i <= numel(text) && text(i) == ','
            i = i + 1;
        elseif i <= numel(text) && text(i) == '}'
            i = i + 1;
            return;
        else
            error('opensysml:json', 'expected , or } at %d', i);
        end
    end
end

function [list, i] = readArray(text, i)
    list = {};
    i = skipWs(text, i);
    if i <= numel(text) && text(i) == ']'
        i = i + 1;
        return;
    end
    while true
        [child, i] = readValue(text, i);
        list{end+1} = child;
        i = skipWs(text, i);
        if i <= numel(text) && text(i) == ','
            i = i + 1;
        elseif i <= numel(text) && text(i) == ']'
            i = i + 1;
            return;
        else
            error('opensysml:json', 'expected , or ] at %d', i);
        end
    end
end

function [s, i] = readString(text, i)
    % text(i) == '"'
    i = i + 1;
    out = '';
    while i <= numel(text)
        c = text(i);
        if c == '"'
            s = out;
            i = i + 1;
            return;
        elseif c == '\'
            i = i + 1;
            esc = text(i);
            switch esc
                case '"', out(end+1) = '"';
                case '\', out(end+1) = '\';
                case '/', out(end+1) = '/';
                case 'b', out(end+1) = char(8);
                case 'f', out(end+1) = char(12);
                case 'n', out(end+1) = sprintf('\n');
                case 'r', out(end+1) = sprintf('\r');
                case 't', out(end+1) = sprintf('\t');
                case 'u'
                    code = text(i+1:i+4);
                    out(end+1) = char(hex2dec(code));
                    i = i + 4;
                otherwise
                    error('opensysml:json', 'bad escape \\%s at %d', esc, i);
            end
            i = i + 1;
        else
            out(end+1) = c;
            i = i + 1;
        end
    end
    error('opensysml:json', 'unterminated string');
end

function [n, i] = readNumber(text, i)
    j = i;
    while j <= numel(text) && ismember(text(j), '-+0123456789.eE')
        j = j + 1;
    end
    n = str2double(text(i:j-1));
    if isnan(n)
        error('opensysml:json', 'bad number at %d', i);
    end
    i = j;
end
