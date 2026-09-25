function scenarios = load_scenarios(scenariosDir)
%LOAD_SCENARIOS Every scenario of every *.json file under dir, in file then
%declaration order, with duplicate ids refused.

    files = {};
    listing = ls_dir(scenariosDir);
    for i = 1:numel(listing)
        [~, ~, ext] = fileparts(listing{i});
        if strcmp(ext, '.json'), files{end+1} = listing{i}; end
    end
    files = sort(files);
    scenarios = {};
    seen = containers.Map();
    for i = 1:numel(files)
        path = fullfile(scenariosDir, files{i});
        suite = json_tree(fileread(path));
        entries = suite('scenarios');
        for j = 1:numel(entries)
            sc = entries{j};
            id = ''; rpc = '';
            if isKey(sc, 'id'), id = sc('id'); end
            if isKey(sc, 'rpc'), rpc = sc('rpc'); end
            if isempty(id) || isempty(rpc)
                error('opensysml:conformance', '%s: scenario needs id and rpc', path);
            end
            if isKey(seen, id)
                error('opensysml:conformance', 'duplicate scenario id "%s"', id);
            end
            seen(id) = true;
            scenarios{end+1} = sc;
        end
    end
    if isempty(scenarios)
        error('opensysml:conformance', 'no scenario files in %s', scenariosDir);
    end
end

function listing = ls_dir(scenariosDir)
    d = dir(scenariosDir);
    listing = {};
    for i = 1:numel(d)
        if ~d(i).isdir && ~ismember(d(i).name, {'.', '..'})
            listing{end+1} = d(i).name;
        end
    end
end
