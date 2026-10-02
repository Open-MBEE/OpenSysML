function path = absolutePath(path)
%ABSOLUTEPATH An absolute, lexically normalized spelling of a path: relative
%   to the working directory, with . and .. segments resolved.

    path = char(path);
    if ~is_absolute(path)
        path = fullfile(pwd, path);
    end
    path = strrep(path, '\', '/');
    root = '';
    if ispc && numel(path) >= 2 && path(2) == ':'
        root = path(1:2);
        path = path(3:end);
    end
    segments = strsplit(path, '/');
    kept = {};
    for i = 1:numel(segments)
        segment = segments{i};
        if isempty(segment) || strcmp(segment, '.')
            continue;
        elseif strcmp(segment, '..')
            if ~isempty(kept), kept(end) = []; end
        else
            kept{end+1} = segment; %#ok<AGROW>
        end
    end
    path = [root '/' strjoin(kept, '/')];
    if ~ispc
        return;
    end
    path = strrep(path, '/', filesep);
end

function tf = is_absolute(path)
    tf = ~isempty(path) && (path(1) == '/' || path(1) == '\' || ...
        (numel(path) >= 2 && path(2) == ':'));
end
