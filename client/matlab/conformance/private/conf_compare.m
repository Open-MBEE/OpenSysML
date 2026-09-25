function failures = conf_compare(expect, actual)
%CONF_COMPARE The conformance expectation rules over json_tree values:
%response compared field by field (a named list must match exactly in
%length; an absent field satisfies an all-default expectation), reals within
%relative 1e-9, an expected integer equal to the int64 string the wire
%writes, plus non_empty / absent / contains / contains_all / counts /
%min_counts paths.

    failures = {};
    if isKey(expect, 'response') && ~isempty(expect('response'))
        failures = compare_value(expect('response'), actual, '$', failures);
    end
    if isKey(expect, 'non_empty')
        for i = 1:numel(expect('non_empty'))
            path = expect('non_empty'){i};
            [got, found] = conf_lookup(actual, path);
            if ~found
                failures{end+1} = sprintf('%s is absent', path);
            elseif conf_is_default(got)
                failures{end+1} = sprintf('%s is empty', path);
            end
        end
    end
    if isKey(expect, 'absent')
        for i = 1:numel(expect('absent'))
            path = expect('absent'){i};
            [got, found] = conf_lookup(actual, path);
            if found && ~conf_is_default(got)
                failures{end+1} = sprintf('%s is not absent/default: %s', path, show(got));
            end
        end
    end
    if isKey(expect, 'contains')
        m = expect('contains');
        keys = m.keys;
        for i = 1:numel(keys)
            path = keys{i}; needle = m(path);
            [got, found] = conf_lookup(actual, path);
            if ischar(got) && ~isempty(strfind(got, needle))
            elseif ~found
                failures{end+1} = sprintf('%s is absent', path);
            else
                failures{end+1} = sprintf('%s=%s does not contain "%s"', path, show(got), needle);
            end
        end
    end
    if isKey(expect, 'contains_all')
        m = expect('contains_all');
        keys = m.keys;
        for i = 1:numel(keys)
            path = keys{i}; needles = m(path);
            if ~iscell(needles), needles = {needles}; end
            failures = check_contains_all(path, needles, actual, failures);
        end
    end
    for field = {'counts', 'min_counts'}
        f = field{1};
        if isKey(expect, f)
            m = expect(f);
            keys = m.keys;
            for i = 1:numel(keys)
                path = keys{i}; wanted = m(path);
                failures = check_count(path, wanted, strcmp(f, 'min_counts'), actual, failures);
            end
        end
    end
end

function failures = check_contains_all(path, needles, actual, failures)
    if any(strcmp(strsplit(path, '.'), '*'))
        failures = check_members_at(path, needles, actual, failures);
        return;
    end
    [got, found] = conf_lookup(actual, path);
    if ischar(got)
        for i = 1:numel(needles)
            if isempty(strfind(got, needles{i}))
                failures{end+1} = sprintf('%s does not contain "%s"', path, needles{i});
            end
        end
    elseif iscell(got)
        for i = 1:numel(needles)
            if ~any(cellfun(@(v) isequaln(v, needles{i}), got))
                failures{end+1} = sprintf('%s does not contain member "%s"', path, needles{i});
            end
        end
    else
        failures = check_members_at(path, needles, actual, failures);
    end
end

function failures = check_members_at(path, needles, actual, failures)
    values = conf_values_at(actual, path);
    if isempty(values)
        failures{end+1} = sprintf('%s is absent or not text/list', path);
        return;
    end
    for i = 1:numel(needles)
        if ~any(cellfun(@(v) ischar(v) && strcmp(v, needles{i}), values))
            failures{end+1} = sprintf('%s does not contain member "%s"', path, needles{i});
        end
    end
end

function failures = check_count(path, wanted, minimum, actual, failures)
    [got, found] = conf_lookup(actual, path);
    if ~found
        count = 0;  % an absent list or map holds zero entries
    elseif iscell(got) || isa(got, 'containers.Map')
        count = numel(got);
        if isa(got, 'containers.Map'), count = got.Count; end
    else
        failures{end+1} = sprintf('%s is not a list/map', path);
        return;
    end
    if (minimum && count < wanted) || (~minimum && count ~= wanted)
        if minimum, rel = 'at least'; else, rel = 'exactly'; end
        failures{end+1} = sprintf('%s has %d entries, want %s %d', path, count, rel, wanted);
    end
end

function failures = compare_value(expected, actual, path, failures)
    if isa(expected, 'containers.Map') && isa(actual, 'containers.Map')
        keys = expected.keys;
        for i = 1:numel(keys)
            key = keys{i};
            [got, found] = conf_lookup(actual, key);
            if ~found
                if ~conf_default_expectation(expected(key))
                    failures{end+1} = sprintf('%s.%s is absent', path, key);
                end
            else
                failures = compare_value(expected(key), got, [path '.' key], failures);
            end
        end
    elseif iscell(expected) && iscell(actual)
        if numel(expected) ~= numel(actual)
            failures{end+1} = sprintf('%s has %d entries, want %d', path, numel(actual), numel(expected));
            return;
        end
        for i = 1:numel(expected)
            failures = compare_value(expected{i}, actual{i}, sprintf('%s.%d', path, i-1), failures);
        end
    elseif isnumeric(expected) && ~islogical(expected) && isscalar(expected)
        if ~numbers_equal(expected, actual)
            failures{end+1} = sprintf('%s: got %s, want %s', path, show(actual), show(expected));
        end
    else
        if ~tree_equal(expected, actual)
            failures{end+1} = sprintf('%s: got %s, want %s', path, show(actual), show(expected));
        end
    end
end

function tf = numbers_equal(expected, actual)
    tf = false;
    if ischar(actual)
        n = str2double(actual);
        if ~isnan(n) && n == floor(n)
            tf = (int64(n) == int64(expected));
            return;
        end
    end
    if isnumeric(actual) && isscalar(actual) && ~islogical(actual)
        if expected == actual
            tf = true; return;
        end
        scale = max(abs(expected), abs(actual));
        tf = scale ~= 0 && abs(expected - actual) / scale <= 1e-9;
    end
end

function tf = tree_equal(a, b)
    if isa(a, 'containers.Map') && isa(b, 'containers.Map')
        if a.Count ~= b.Count, tf = false; return; end
        keys = a.keys;
        tf = true;
        for i = 1:numel(keys)
            if ~isKey(b, keys{i}) || ~tree_equal(a(keys{i}), b(keys{i}))
                tf = false; return;
            end
        end
    elseif iscell(a) && iscell(b)
        if numel(a) ~= numel(b), tf = false; return; end
        tf = true;
        for i = 1:numel(a)
            if ~tree_equal(a{i}, b{i}), tf = false; return; end
        end
    else
        tf = isequaln(a, b);
    end
end

function s = show(v)
    if ischar(v), s = v;
    elseif isnumeric(v) || islogical(v), s = mat2str(v);
    else, s = json_write(v); end
end
