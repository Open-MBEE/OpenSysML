function values = runCapabilities(conn, schedule, performer, trace)
%RUNCAPABILITIES Require the capabilities named by a run's options.

    if nargin < 4, trace = false; end
    values = {};
    if ~isempty(schedule)
        values{end+1} = 'schedule';
        if explores(schedule), values{end+1} = 'schedule_explore'; end
    end
    if ~isempty(performer), values{end+1} = 'performer'; end
    if trace, values{end+1} = 'state_trace'; end
    for i = 1:numel(values), conn.require(values{i}); end
end

function tf = explores(schedule)
    schedule = char(schedule);
    tf = strcmp(schedule, 'explore') || strncmp(schedule, 'explore:', 8);
end
