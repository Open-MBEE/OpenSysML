function bin = resolveBinary()
%RESOLVEBINARY Locate a sysml-grpc binary; never a download.
%   $OPENSYSML_GRPC_BINARY, then ~/.opensysml/bin/sysml-grpc
%   (sysml-grpc.exe on Windows), then PATH.

    bin = getenv('OPENSYSML_GRPC_BINARY');
    if ~isempty(bin), return; end
    if ispc
        name = 'sysml-grpc.exe';
    else
        name = 'sysml-grpc';
    end
    home = getenv('HOME');
    if isempty(home) && ispc, home = getenv('USERPROFILE'); end
    if ~isempty(home)
        cached = fullfile(home, '.opensysml', 'bin', name);
        if exist(cached, 'file'), bin = cached; return; end
    end
    [rc, out] = system(sprintf('which %s 2>/dev/null', name));
    if rc == 0 && ~isempty(strtrim(out))
        bin = strtrim(out); return;
    end
    if ispc
        w = which(name);
        if ~isempty(w), bin = w; return; end
    end
    error('opensysml:transport', ['no sysml-grpc binary found; set OPENSYSML_GRPC_BINARY, ' ...
        'install to ~/.opensysml/bin/%s, or put it on PATH'], name);
end
