function model = parseSource(conn, content, varargin)
%PARSESOURCE Parse inline content; 'name' defaults to "inline.sysml".

    name = 'inline.sysml'; language = '';
    for i = 1:2:numel(varargin)
        switch varargin{i}
            case 'name', name = varargin{i+1};
            case 'language', language = varargin{i+1};
        end
    end
    model = opensysml.parseSources(conn, {{name, content}}, 'language', language);
end
