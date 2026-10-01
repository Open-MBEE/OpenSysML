function requireEngine(conn, engine)
%REQUIREENGINE Gate explicit engine selections and exploration.

    if isempty(engine) || strcmp(engine, 'auto'), return; end
    conn.require('engines');
    if strcmp(engine, 'explore'), conn.require('schedule_explore'); end
end
