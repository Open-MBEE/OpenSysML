function [status, contentType, bodyText] = httpPost(url, requestJsonText, timeoutSec, token)
%POST one Connect-JSON call and return the raw answer pieces.
%   MATLAB uses matlab.net.http; where that is absent (GNU Octave) curl answers instead.
%   A non-empty token is sent as Authorization: Bearer TOKEN.

    if nargin < 4, token = ''; end
    token = char(token);
    if ~isempty(token) && isempty(regexp(token, '^[A-Za-z0-9._~+/-]+=*$', 'once'))
        opensysml.internal.raise('opensysml:transport', ...
            'the bearer token is not RFC 6750 token syntax (letters, digits, -._~+/ and trailing =)');
    end
    if exist('matlab.net.http.RequestMessage', 'class')
        [status, contentType, bodyText] = post_matlab(url, requestJsonText, timeoutSec, token);
    else
        [status, contentType, bodyText] = post_curl(url, requestJsonText, timeoutSec, token);
    end
end

function [status, contentType, bodyText] = post_matlab(url, requestJsonText, timeoutSec, token)
    import matlab.net.http.*
    headers = HeaderField('Content-Type', 'application/json');
    if ~isempty(token)
        headers = [headers HeaderField('Authorization', ['Bearer ' token])];
    end
    req = RequestMessage('POST', headers, MessageBody(requestJsonText));
    try
        resp = req.send(url, HTTPOptions('ConvertResponse', false, 'ConnectTimeout', timeoutSec, ...
                                        'ResponseTimeout', timeoutSec));
    catch e
        description = lower([e.identifier ' ' e.message]);
        if ~isempty(strfind(description, 'timeout')) || ...
                ~isempty(strfind(description, 'timed out'))
            opensysml.internal.raise('opensysml:connect:serviceTimeout', ...
                sprintf('HTTP request timed out: %s', e.message));
        end
        opensysml.internal.raise('opensysml:transport', ...
            sprintf('HTTP request failed: %s', e.message));
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

function [status, contentType, bodyText] = post_curl(url, requestJsonText, timeoutSec, token)
    if isempty(regexp(url, '^https?://[A-Za-z0-9._~%:\[\]\-]+(/[A-Za-z0-9._~%\-/]*)?$', 'once'))
        error('opensysml:transport', 'refusing to send to an address with shell metacharacters: %s', url);
    end
    % The token goes to curl through a file only this user can read, never
    % through the command line, which every local user can list.
    authorization = '';
    if ~isempty(token)
        headerFile = [tempname '.hdr'];
        if any(headerFile == '''') || any(headerFile == sprintf('\n'))
            error('opensysml:transport', 'refusing a header file path that breaks quoting: %s', headerFile);
        end
        % Created private from the first instant (umask 077), never widened
        % later; on Windows the user's temporary directory is private itself.
        if ~ispc() && system(sprintf('umask 077 && : > ''%s''', headerFile)) ~= 0
            error('opensysml:transport', 'could not create a private header file: %s', headerFile);
        end
        fid = fopen(headerFile, 'w');
        fwrite(fid, sprintf('Authorization: Bearer %s', token));
        fclose(fid);
        headerCleanup = onCleanup(@() delete(headerFile)); %#ok<NASGU>
        authorization = sprintf('-H @''%s'' ', headerFile);
    end
    % curl writes status and content type after the marker, on lines of their own.
    marker = sprintf('\n__OPENSYSML_STATUS__\n');
    bodyFile = [tempname '.json'];
    if any(bodyFile == '''') || any(bodyFile == sprintf('\n'))
        error('opensysml:transport', 'refusing a body file path that breaks quoting: %s', bodyFile);
    end
    fid = fopen(bodyFile, 'w');
    fwrite(fid, requestJsonText);
    fclose(fid);
    cleanup = onCleanup(@() delete(bodyFile));
    timeoutSec = max(double(timeoutSec), 0.001);
    cmd = sprintf(['curl -sS --max-time %.3f -X POST -H "Content-Type: application/json" %s' ...
                   '--data-binary @''%s'' -w "%s%%{http_code}\n%%{content_type}" ''%s'''], ...
                  timeoutSec, authorization, bodyFile, marker, url);
    [rc, out] = system(cmd);
    if rc ~= 0
        idx = strfind(out, marker);
        if ~isempty(idx), out = out(1:idx(1)-1); end
        detail = strtrim(out);
        if isempty(detail), detail = 'no answer'; end
        if rc == 28
            opensysml.internal.raise('opensysml:connect:serviceTimeout', ...
                sprintf('HTTP request timed out after %.3f seconds: %s', timeoutSec, detail));
        end
        opensysml.internal.raise('opensysml:transport', ...
            sprintf('HTTP request failed (curl exit %d): %s', rc, detail));
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
