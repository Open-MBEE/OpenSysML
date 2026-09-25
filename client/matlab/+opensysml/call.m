function out = call(conn, method, request)
%CALL Post a Connect-JSON request and return the decoded answer.
%   A non-200 status whose body is a Connect {"code","message"} object raises
%   'opensysml:connect'; a transport failure, or an answer that is not JSON
%   (the Content-Type is checked before parsing, per the wire contract),
%   raises 'opensysml:transport'. The decoded answer is jsondecode output.

    [status, contentType, bodyText] = opensysml.callRaw(conn, method, jsonencode(request));
    isJson = ~isempty(strfind(lower(contentType), 'application/json'));
    if status ~= 200
        if isJson
            out = jsondecode(bodyText);
            code = 'unknown'; message = '';
            if isfield(out, 'code'), code = out.code; end
            if isfield(out, 'message'), message = out.message; end
            error('opensysml:connect', 'connect %s (HTTP %d): %s', code, status, message);
        end
        error('opensysml:transport', '%s answered HTTP %d with a non-JSON body', method, status);
    end
    if ~isJson
        error('opensysml:transport', '%s answered HTTP 200 with a non-JSON body', method);
    end
    out = jsondecode(bodyText);
end
