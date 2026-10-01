function values = listEngines(conn)
%LISTENGINES List analysis engines registered with the service.

    conn.require('engines');
    raw = opensysml.internal.checkError(opensysml.call(conn, 'ListEngines', ...
        struct(), {'engines'}), 'ListEngines');
    items = fieldOr(raw, 'engines', {});
    if isstruct(items), items = num2cell(items(:)'); end
    if ~iscell(items), items = {}; end
    values = cell(1, numel(items));
    for i = 1:numel(items), values{i} = opensysml.EngineInfo(items{i}); end
end

function value = fieldOr(raw, name, fallback)
    value = fallback;
    if isstruct(raw) && isfield(raw, name), value = raw.(name); end
end
