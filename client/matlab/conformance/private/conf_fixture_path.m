function path = conf_fixture_path(dir, name)
%CONF_FIXTURE_PATH A fixture name inside dir, never outside it.

    if ~isempty(strfind(name, '\'))
        error('opensysml:conformance', 'fixture "%s" is outside %s', name, dir);
    end
    if ~isempty(regexp(name, '(^|/)\.\.(/|$)', 'once')) || ~isempty(regexp(name, '^[a-zA-Z]:', 'once')) || strncmp(name, '/', 1)
        error('opensysml:conformance', 'fixture "%s" is outside %s', name, dir);
    end
    parts = strsplit(name, '/');
    path = dir;
    for i = 1:numel(parts)
        path = fullfile(path, parts{i});
    end
end
