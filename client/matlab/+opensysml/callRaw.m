function [status, contentType, bodyText] = callRaw(conn, method, requestJsonText)
%CALLRAW Post raw Connect-JSON request text; return the raw answer pieces.
%   Public for tooling: the conformance runner sends the scenario's request
%   exactly as written and compares the answer it gets back.

    url = sprintf('%s/sysml.SysMLService/%s', conn.base, method);
    [status, contentType, bodyText] = opensysml.internal.httpPost(url, requestJsonText, conn.timeout);
end
