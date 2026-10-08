function connectError(code, message, status, method, conn, capabilities, request)
%CONNECTERROR Translate a Connect status into the public client identifier.

    if nargin < 6 || isempty(capabilities), capabilities = {}; end
    if nargin < 7, request = struct(); end
    code = char(code);
    message = char(message);
    if isempty(message), message = sprintf('the sysml-grpc service failed the call with %s', code); end
    lowered = lower(message);
    details = struct('code', code, 'httpStatus', status, 'service', serviceOrigin(conn));

    switch code
        case 'not_found'
            if ~isempty(strfind(lowered, 'file not found')) || ~isempty(strfind(lowered, 'no such file'))
                identifier = 'opensysml:connect:modelFileNotFound';
            elseif ~isempty(strfind(lowered, 'model not found'))
                identifier = 'opensysml:connect:modelNotFound';
            elseif ~isempty(strfind(lowered, 'symbol not found'))
                identifier = 'opensysml:connect:symbolNotFound';
            elseif any(strcmp(method, {'ParseFile', 'ParseSources'})) && hasFilePath(request)
                identifier = 'opensysml:connect:modelFileNotFound';
            elseif any(strcmp(method, {'GetSymbol', 'RunDocumentQuery', 'RenderDocument', 'RenderView'}))
                identifier = 'opensysml:connect:symbolNotFound';
            else
                identifier = 'opensysml:connect:modelNotFound';
            end
        case {'invalid_argument', 'failed_precondition', 'out_of_range'}
            identifier = 'opensysml:connect:invalidRequest';
        case 'unavailable'
            identifier = 'opensysml:connect:unavailable';
        case {'deadline_exceeded', 'canceled'}
            identifier = 'opensysml:connect:serviceTimeout';
        case 'unimplemented'
            capability = refusedCapability(message, capabilities);
            if ~isempty(capability)
                details.capability = capability;
                identifier = 'opensysml:missingCapability';
                message = opensysml.internal.missingCapabilityMessage( ...
                    capability, conn.describe());
            else
                identifier = 'opensysml:connect:unsupportedOperation';
            end
        otherwise
            identifier = 'opensysml:connect:service';
    end
    opensysml.internal.raise(identifier, message, {}, details);
end

function tf = hasFilePath(request)
    tf = isfield(request, 'filePath');
    if tf || ~isfield(request, 'documents') || isempty(request.documents), return; end
    documents = request.documents;
    if isstruct(documents), documents = num2cell(documents); end
    if iscell(documents)
        for i = 1:numel(documents)
            if isstruct(documents{i}) && isfield(documents{i}, 'filePath')
                tf = true;
                return;
            end
        end
    end
end

function capability = refusedCapability(message, capabilities)
    capability = '';
    if isempty(capabilities), return; end
    token = regexp(message, 'capability\s+"([^"]+)"', 'tokens', 'once');
    if ~isempty(token) && any(strcmp(capabilities, token{1}))
        capability = token{1};
    else
        capability = capabilities{1};
    end
end

function origin = serviceOrigin(conn)
    origin = conn.origin;
    if isempty(origin), origin = conn.base; end
end
