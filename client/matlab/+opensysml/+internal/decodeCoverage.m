function coverage = decodeCoverage(raw)
%DECODECOVERAGE Decode the states entered and transitions taken across every run.
%   The result is a struct with cell arrays `states` and `transitions`, each entry a
%   struct with `path` (the state or transition under the instance running the
%   machine), `declaration` (the same under its declaration) and `runs` (how many
%   runs, or schedules for a check, entered or took it).

    coverage = struct('states', {entries(fieldOr(raw, 'states', {}))}, ...
        'transitions', {entries(fieldOr(raw, 'transitions', {}))});
end

function out = entries(raw)
    cells = toCells(raw);
    out = cell(1, numel(cells));
    for i = 1:numel(cells)
        out{i} = struct('path', char(fieldOr(cells{i}, 'path', '')), ...
            'declaration', char(fieldOr(cells{i}, 'declaration', '')), ...
            'runs', int64(fieldOr(cells{i}, 'runs', 0)));
    end
end

function values = toCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {};
    end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
