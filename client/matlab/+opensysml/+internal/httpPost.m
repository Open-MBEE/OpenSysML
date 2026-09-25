function [status, contentType, bodyText] = httpPost(url, requestJsonText, timeoutSec)
%POST one Connect-JSON call and return the raw answer pieces.
%   MATLAB uses matlab.net.http; where that is absent (GNU Octave) curl answers instead.

    if exist('matlab.net.http.RequestMessage', 'class')
        [status, contentType, bodyText] = post_matlab(url, requestJsonText, timeoutSec);
    else
        [status, contentType, bodyText] = post_curl(url, requestJsonText, timeoutSec);
    end
end

function [status, contentType, bodyText] = post_matlab(url, requestJsonText, timeoutSec)
    import matlab.net.http.*
    req = RequestMessage('POST', ...
        HeaderField('Content-Type', 'application/json'), ...
        MessageBody(requestJsonText));
    try
        resp = req.send(url, HTTPOptions('ConvertResponse', false, 'ConnectTimeout', timeoutSec));
    catch e
        error('opensysml:transport', 'HTTP request failed: %s', e.message);
    end
    status = double(resp.StatusCode);
    contentType = '';
    for h = resp.Header
        if strcmpi(h.Name, 'Content-Type')
            contentType = char(h.Value);
        end
    end
    bodyText = char(resp.Body.Data);
end

function [status, contentType, bodyText] = post_curl(url, requestJsonText, timeoutSec)
    % curl writes status and content type after the marker, on lines of their own.
    marker = sprintf('\n__OPENSYSML_STATUS__\n');
    bodyFile = [tempname '.json'];
    fid = fopen(bodyFile, 'w');
    fwrite(fid, requestJsonText);
    fclose(fid);
    cleanup = onCleanup(@() delete(bodyFile));
    cmd = sprintf(['curl -sS --max-time %d -X POST -H "Content-Type: application/json" ' ...
                   '--data-binary @"%s" -w "%s%%{http_code}\n%%{content_type}" "%s"'], ...
                  round(timeoutSec), bodyFile, marker, url);
    [rc, out] = system(cmd);
    if rc ~= 0
        error('opensysml:transport', 'HTTP request failed: %s', strtrim(out));
    end
    idx = strfind(out, marker);
    if isempty(idx)
        error('opensysml:transport', 'curl returned an unparseable answer');
    end
    bodyText = out(1:idx-1);
    rest = strsplit(out(idx+numel(marker):end), '\n');
    status = str2double(rest{1});
    if isnan(status)
        error('opensysml:transport', 'curl reported no HTTP status: %s', strtrim(out));
    end
    if numel(rest) > 1, contentType = strtrim(rest{2}); else, contentType = ''; end
end
