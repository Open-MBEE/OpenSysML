function values = listEngines(conn, varargin)
%LISTENGINES List analysis engines registered with the service.
%   listEngines(conn, 'probe', true) starts each external engine's process
%   once and checks its handshake against its manifest, reporting the outcome
%   as its status, as sysml -engines -probe does.

    options = opensysml.internal.nameValueOptions(struct('probe', false), varargin, 'listEngines');
    conn.require('engines');
    request = struct();
    if islogical(options.probe) && isscalar(options.probe) && options.probe
        request.probe = true;
    elseif ~(islogical(options.probe) && isscalar(options.probe))
        opensysml.internal.raise('opensysml:argument', 'probe must be true or false');
    end
    raw = opensysml.internal.checkError(opensysml.call(conn, 'ListEngines', ...
        request, {'engines'}), 'ListEngines');
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
