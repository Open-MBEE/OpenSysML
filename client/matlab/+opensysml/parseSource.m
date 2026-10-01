function model = parseSource(conn, content, varargin)
%PARSESOURCE Parse inline content; 'name' defaults to "inline.sysml".

    name = 'inline.sysml';
    language = [];
    options = {};
    if mod(numel(varargin), 2) ~= 0
        opensysml.internal.raise('opensysml:argument', ...
            'parseSource options must be name-value pairs');
    end
    for i = 1:2:numel(varargin)
        key = char(varargin{i});
        switch key
            case 'name'
                name = varargin{i+1};
            case 'language'
                language = varargin{i+1};
            case {'strict','strictConformance','raiseForErrors'}
                options(end+1:end+2) = varargin(i:i+1);
            otherwise
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('parseSource has no option ''%s''', key));
        end
    end
    document = opensysml.SourceDocument.inline(name, content, language);
    model = opensysml.parseSources(conn, {document}, options{:});
end
