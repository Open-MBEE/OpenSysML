function conn = connect(varargin)
%CONNECT Dial $OPENSYSML_SERVICE when set, else start a private child.

    address = getenv('OPENSYSML_SERVICE');
    if ~isempty(address)
        conn = opensysml.external(address, varargin{:});
    else
        conn = opensysml.private(varargin{:});
    end
end
