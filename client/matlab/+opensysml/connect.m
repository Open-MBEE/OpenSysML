function conn = connect()
%CONNECT Dial $OPENSYSML_SERVICE when set, else start a private child.

    address = getenv('OPENSYSML_SERVICE');
    if ~isempty(address)
        conn = opensysml.external(address);
    else
        conn = opensysml.private();
    end
end
