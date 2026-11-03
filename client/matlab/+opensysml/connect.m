function conn = connect(varargin)
%CONNECT Dial $OPENSYSML_SERVICE when set, else start a private child.

    address = getenv('OPENSYSML_SERVICE');
    if ~isempty(address)
        conn = opensysml.external(address, varargin{:});
    else
        args = withoutToken(varargin);
        conn = opensysml.private(args{:});
    end
end

function args = withoutToken(args)
%WITHOUTTOKEN A private child on loopback needs no bearer token; drop the option.
    keep = true(1, numel(args));
    for i = 1:2:numel(args) - 1
        if strcmp(char(args{i}), 'token')
            keep(i:i+1) = false;
        end
    end
    args = args(keep);
end
