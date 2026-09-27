function out = call(conn, method, request)
%CALL Post a Connect-JSON request and return the decoded answer. A non-200
%   Connect body raises 'opensysml:connect'; a non-JSON answer 'opensysml:transport'.

    [status, contentType, bodyText] = opensysml.callRaw(conn, method, jsonencode(request));
    isJson = ~isempty(strfind(lower(contentType), 'application/json'));
    if status ~= 200
        if isJson
            out = decode_body(bodyText, method, status);
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
    out = decode_body(bodyText, method, status);
end

function out = decode_body(bodyText, method, status)
    try
        out = jsondecode(bodyText);
    catch
        error('opensysml:transport', '%s answered HTTP %d with undecodable JSON', method, status);
    end
end
