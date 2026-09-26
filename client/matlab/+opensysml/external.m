function conn = external(address)
%EXTERNAL Connect to a service somebody else runs; close does not stop it.

    conn = opensysml.Connection();
    a = strtrim(char(address));
    if ~strncmp(a, 'http://', 7) && ~strncmp(a, 'https://', 8)
        a = ['http://' a];
    end
    conn.base = regexprep(a, '/+$', '');
end
