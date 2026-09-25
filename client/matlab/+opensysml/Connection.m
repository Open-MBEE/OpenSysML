classdef Connection < handle
% Connection to a sysml-grpc service over Connect-JSON.

    properties
        base            % 'http(s)://host:port' without a trailing slash
        privateService = false   % true when this process started the child
        process = []             % java.lang.Process for a private child
        childStdin = []          % the child's stdin stream, held open
        timeout = 30             % request timeout in seconds
    end
    properties (Access = private)
        info = struct()          % cached GetServerInfo answer
        infoSet = false
    end

    methods
        function [version, capabilities] = serverInfo(conn)
            if ~conn.infoSet
                conn.info = opensysml.call(conn, 'GetServerInfo', struct());
                conn.infoSet = true;
            end
            version = '';
            if isfield(conn.info, 'version'), version = conn.info.version; end
            capabilities = {};
            if isfield(conn.info, 'capabilities')
                capabilities = cellstr(conn.info.capabilities);
            end
        end

        function tf = hasCapability(conn, name)
            [~, capabilities] = conn.serverInfo();
            tf = any(strcmp(capabilities, name));
        end

        function close(conn)
            if ~isempty(conn.childStdin)
                try, conn.childStdin.close(); catch, end
            end
            if ~isempty(conn.process)
                try
                    % the child exits at end of file on its stdin; destroy() is
                    % the fallback, not the mechanism
                    conn.process.destroy();
                    conn.process.waitFor();
                catch
                end
            end
            conn.childStdin = [];
            conn.process = [];
        end

        function delete(conn)
            try, conn.close(); catch, end
        end
    end
end
