function out = call(conn, method, request, capabilities)
%CALL Post a Connect-JSON request and return the decoded answer. A non-200
%   Connect body raises a method-appropriate client error.

    if nargin < 4, capabilities = {}; end
    if ~strcmp(method, 'GetServerInfo') && ...
            (~isempty(conn.expectedVersion) || ~isempty(conn.expectedCapabilities))
        conn.serverInfo();
    end
    try
        [status, contentType, bodyText] = opensysml.callRaw(conn, method, jsonencode(request));
    catch e
        if strncmp(e.identifier, 'opensysml:', 10)
            opensysml.internal.raise(e.identifier, e.message, {}, ...
                struct('service', serviceOrigin(conn)));
        end
        rethrow(e);
    end
    isJson = ~isempty(strfind(lower(contentType), 'application/json'));
    if status ~= 200
        if isJson
            out = decode_body(bodyText, method, status);
            code = 'unknown'; message = '';
            if isfield(out, 'code'), code = out.code; end
            if isfield(out, 'message'), message = out.message; end
            opensysml.internal.connectError(code, message, status, method, conn, capabilities, request);
        end
        opensysml.internal.raise('opensysml:transport', ...
            sprintf('%s answered HTTP %d with a non-JSON body', method, status), {}, ...
            struct('httpStatus', status, 'service', serviceOrigin(conn)));
    end
    if ~isJson
        opensysml.internal.raise('opensysml:transport', ...
            sprintf('%s answered HTTP 200 with a non-JSON body', method), {}, ...
            struct('httpStatus', status, 'service', serviceOrigin(conn)));
    end
    out = decode_body(bodyText, method, status);
end

function out = decode_body(bodyText, method, status)
    try
        out = opensysml.internal.decodeJson(bodyText);
    catch
        opensysml.internal.raise('opensysml:transport', ...
            sprintf('%s answered HTTP %d with undecodable JSON', method, status), {}, ...
            struct('httpStatus', status));
    end
end

function origin = serviceOrigin(conn)
    origin = conn.origin;
    if isempty(origin), origin = conn.base; end
end
