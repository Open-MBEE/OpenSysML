classdef Model < handle
%MODEL A parsed model: the connection, its hash, its parse diagnostics.

    properties
        connection      % opensysml.Connection
        hash = ''       % the model hash the service caches the model under
        diagnostics = {}    % cell of structs: severity, message, file, line, col, code
    end

    methods
        function m = Model(conn, hash, diags)
            m.connection = conn;
            if nargin >= 2, m.hash = hash; end
            if nargin >= 3, m.diagnostics = diags; end
        end
    end
end
