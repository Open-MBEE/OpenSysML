classdef Connection < handle
% Connection to a sysml-grpc service over Connect-JSON.

    properties
        base            % 'http(s)://host:port' without a trailing slash
        origin = ''
        privateService = false   % true when this process started the child
        process = []             % java.lang.Process for a private child
        childStdin = []          % the child's stdin stream, held open
        timeout = 30             % request timeout in seconds
    end
    properties (Hidden)
        expectedVersion = ''
        expectedCapabilities = {}
    end
    properties (Access = private)
        info = struct()          % cached GetServerInfo answer
        infoSet = false
    end

    methods
        function [version, capabilities] = serverInfo(conn)
            if ~conn.infoSet
                try
                    answer = opensysml.call(conn, 'GetServerInfo', struct());
                    version = '';
                    capabilities = {};
                    if isfield(answer, 'version'), version = char(answer.version); end
                    if isfield(answer, 'capabilities')
                        capabilities = toCellstr(answer.capabilities);
                    end
                    conn.info = struct('version', version, 'capabilities', {capabilities}, ...
                                       'answered', true);
                catch e
                    if strcmp(e.identifier, 'opensysml:connect:unsupportedOperation')
                        conn.info = struct('version', '', 'capabilities', {{}}, 'answered', false);
                    else
                        rethrow(e);
                    end
                end
                conn.infoSet = true;
            end
            conn.checkRequirements();
            version = conn.info.version;
            capabilities = conn.info.capabilities;
        end

        function tf = hasCapability(conn, name)
            [~, capabilities] = conn.serverInfo();
            tf = any(strcmp(capabilities, name));
        end

        function text = describe(conn)
            conn.serverInfo();
            text = conn.describeFromInfo();
        end

        function require(conn, capability)
            if ~conn.hasCapability(capability)
                service = conn.describeFromInfo();
                message = missingCapabilityMessage(capability, service);
                opensysml.internal.raise('opensysml:missingCapability', message, {}, ...
                    struct('capability', capability, 'service', service));
            end
        end

        function requireAll(conn, capabilities)
            capabilities = toCellstr(capabilities);
            for i = 1:numel(capabilities)
                conn.require(capabilities{i});
            end
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

        function primeServerInfo(conn, info)
            conn.info = struct('version', '', 'capabilities', {{}}, 'answered', true);
            if isfield(info, 'version'), conn.info.version = char(info.version); end
            if isfield(info, 'capabilities'), conn.info.capabilities = toCellstr(info.capabilities); end
            if isfield(info, 'answered'), conn.info.answered = logical(info.answered); end
            conn.infoSet = true;
            conn.checkRequirements();
        end

        function delete(conn)
            try, conn.close(); catch, end
        end
    end

    methods (Access = private)
        function checkRequirements(conn)
            if isempty(conn.expectedVersion) && isempty(conn.expectedCapabilities), return; end
            version = conn.info.version;
            reasons = {};
            if ~isempty(conn.expectedVersion)
                if ~conn.info.answered
                    reasons{end+1} = sprintf('it did not answer GetServerInfo, so it cannot be shown to be the %s that was asked for', conn.expectedVersion);
                elseif ~strcmp(version, conn.expectedVersion)
                    if isempty(version), version = 'unknown'; end
                    reasons{end+1} = sprintf('it reports version %s, but %s was asked for', version, conn.expectedVersion);
                end
            end
            missing = {};
            for i = 1:numel(conn.expectedCapabilities)
                cap = conn.expectedCapabilities{i};
                if ~any(strcmp(conn.info.capabilities, cap)), missing{end+1} = cap; end
            end
            missing = sort(missing);
            if ~isempty(missing)
                quoted = cellfun(@(x) ['''' x ''''], missing, 'UniformOutput', false);
                noun = 'capabilities';
                if numel(missing) == 1, noun = 'capability'; end
                reasons{end+1} = sprintf('it does not report the %s %s this client requires', ...
                    strjoin(quoted, ', '), noun);
            end
            if isempty(reasons), return; end
            reason = strjoin(reasons, '; ');
            service = conn.describeFromInfo();
            remedy = 'use a service that reports the requested version and capabilities, or remove those requirements';
            message = sprintf('the sysml-grpc service already listening on %s is not the one this client asked for: %s.\n  service: %s\n  fix:     %s', ...
                conn.origin, reason, service, remedy);
            opensysml.internal.raise('opensysml:staleService', message, {}, ...
                struct('service', service, 'reason', reason));
        end

        function text = describeFromInfo(conn)
            if ~conn.info.answered
                text = sprintf('%s (version unknown: too old to answer GetServerInfo, so it predates every capability)', conn.origin);
                return;
            end
            version = conn.info.version;
            if isempty(version), version = 'unknown'; end
            capabilities = sort(conn.info.capabilities);
            if isempty(capabilities), listed = 'none';
            else, listed = strjoin(capabilities, ', ');
            end
            text = sprintf('%s (version %s, capabilities: %s)', conn.origin, version, listed);
        end
    end
end

function values = toCellstr(values)
    if isempty(values)
        values = {};
    elseif ischar(values)
        values = cellstr(values);
    elseif exist('isstring', 'builtin') || exist('isstring', 'file')
        if isstring(values), values = cellstr(values); end
    end
    if ~iscell(values), values = cellstr(values); end
    values = cellfun(@char, values, 'UniformOutput', false);
end

function message = missingCapabilityMessage(capability, service)
    remedy = sprintf(['run a sysml-grpc whose GetServerInfo reports ''%s'': build one with `make build-grpc` and start it yourself, ' ...
        'or point $OPENSYSML_GRPC_BINARY at a release that has it'], capability);
    message = sprintf(['the sysml-grpc service does not support the ''%s'' capability, which this operation requires.\n' ...
        '  service: %s\n  fix:     %s'], capability, service, remedy);
end
