function raise(identifier, message, varargin)
%RAISE Record a client error and raise it with its public identifier.

    details = struct();
    diagnostics = {};
    if ~isempty(varargin)
        if iscell(varargin{1})
            diagnostics = varargin{1};
            if numel(varargin) > 1 && isstruct(varargin{2}), details = varargin{2}; end
        elseif isstruct(varargin{1})
            details = varargin{1};
            if isfield(details, 'diagnostics'), diagnostics = details.diagnostics; end
            if numel(varargin) > 1 && iscell(varargin{2}), diagnostics = varargin{2}; end
        end
    end
    if isstruct(details) && isfield(details, 'diagnostics') && isempty(diagnostics)
        diagnostics = details.diagnostics;
    end
    entry = struct('identifier', char(identifier), 'message', char(message), ...
                   'diagnostics', {diagnostics}, 'details', details);
    opensysml.internal.errorStore('set', entry);
    error(entry.identifier, '%s', entry.message);
end
