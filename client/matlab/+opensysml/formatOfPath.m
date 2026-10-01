function format = formatOfPath(path)
%FORMATOFPATH Infer a conversion format from a file extension.

    [~, ~, ext] = fileparts(char(path));
    switch lower(ext)
        case {'.sysml', '.kerml'}
            format = 'sysml';
        case {'.ttl', '.turtle'}
            format = 'ttl';
        case '.json'
            format = 'api-json';
        otherwise
            opensysml.internal.raise('opensysml:argument', ...
                sprintf(['cannot tell the format to write ''%s'' as: expected one of ' ...
                '.json, .kerml, .sysml, .ttl, .turtle, or pass toFormat explicitly'], char(path)));
    end
end
