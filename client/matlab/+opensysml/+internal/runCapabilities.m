function values = runCapabilities(conn, schedule, performer, trace, traceCapability)
%RUNCAPABILITIES Require the capabilities named by a run's options.
%   traceCapability names what trace asks for: 'state_trace' (default) or 'action_trace'.

    if nargin < 4, trace = false; end
    if nargin < 5, traceCapability = 'state_trace'; end
    values = {};
    if ~isempty(schedule)
        values{end+1} = 'schedule';
        if explores(schedule), values{end+1} = 'schedule_explore'; end
    end
    if ~isempty(performer), values{end+1} = 'performer'; end
    if trace, values{end+1} = traceCapability; end
    for i = 1:numel(values), conn.require(values{i}); end
end

function tf = explores(schedule)
    schedule = char(schedule);
    tf = strcmp(schedule, 'explore') || strncmp(schedule, 'explore:', 8);
end
