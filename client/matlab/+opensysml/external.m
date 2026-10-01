function conn = external(address, varargin)
%EXTERNAL Connect to a service somebody else runs; close does not stop it.

    conn = opensysml.Connection();
    a = strtrim(char(address));
    if ~strncmp(a, 'http://', 7) && ~strncmp(a, 'https://', 8)
        a = ['http://' a];
    end
    conn.base = regexprep(a, '/+$', '');
    conn.origin = regexprep(regexprep(a, '^https?://', ''), '/+$', '');
    for i = 1:2:numel(varargin)
        if i == numel(varargin)
            opensysml.internal.raise('opensysml:argument', 'name-value options need a value');
        end
        switch char(varargin{i})
            case 'version'
                conn.expectedVersion = char(varargin{i+1});
            case 'capabilities'
                conn.expectedCapabilities = asCellstr(varargin{i+1});
            otherwise
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('unknown external option: %s', char(varargin{i})));
        end
    end
    if ~isempty(conn.expectedVersion) || ~isempty(conn.expectedCapabilities)
        try
            conn.serverInfo();
        catch e
            if ~any(strcmp(e.identifier, {'opensysml:transport', ...
                    'opensysml:connect:unavailable', 'opensysml:connect:serviceTimeout'}))
                rethrow(e);
            end
        end
    end
end

function values = asCellstr(values)
    if isempty(values), values = {};
    elseif ischar(values), values = {values};
    elseif exist('isstring', 'builtin') || exist('isstring', 'file')
        if isstring(values), values = cellstr(values); end
    end
    if ~iscell(values), values = cellstr(values); end
    values = cellfun(@char, values, 'UniformOutput', false);
end
