function [status, contentType, bodyText] = callRaw(conn, method, requestJsonText)
%CALLRAW Post raw Connect-JSON request text and return the raw answer
%   pieces, exactly as written — the path the conformance runner drives.

    url = sprintf('%s/sysml.SysMLService/%s', conn.base, method);
    [status, contentType, bodyText] = opensysml.internal.httpPost(url, requestJsonText, conn.timeout);
end
