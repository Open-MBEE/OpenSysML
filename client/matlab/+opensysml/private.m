function conn = private(varargin)
%PRIVATE Start a child sysml-grpc and connect to it. The spawn needs Java
%   (ProcessBuilder); an Octave built without Java uses opensysml.external.

    binary = ''; timeoutSec = 30;
    for i = 1:2:numel(varargin)
        if strcmp(varargin{i}, 'binary'), binary = varargin{i+1}; end
        if strcmp(varargin{i}, 'timeout'), timeoutSec = varargin{i+1}; end
    end
    if isempty(binary), binary = opensysml.resolveBinary(); end
    if ~exist('java.lang.ProcessBuilder', 'class') && ~isJavaAvailable()
        error('opensysml:transport', ['a private service needs java.lang.ProcessBuilder; ' ...
            'this interpreter was built without Java — start a service yourself and use opensysml.external(address)']);
    end
    args = javaObject('java.util.ArrayList');
    args.add(javaObject('java.lang.String', binary));
    for flag = {'-port', '0', '-health-port', '0', '-report-address', '-exit-with-parent'}
        args.add(javaObject('java.lang.String', flag{1}));
    end
    try
        pb = javaObject('java.lang.ProcessBuilder', args);
        proc = pb.start();
    catch e
        error('opensysml:transport', 'could not start %s: %s', binary, e.message);
    end
    rdr = javaObject('java.io.BufferedReader', ...
                     javaObject('java.io.InputStreamReader', proc.getInputStream()));
    line = ''; complete = false;
    start = tic;
    while ~complete && toc(start) < timeoutSec
        readAny = false;
        while rdr.ready()
            c = rdr.read();
            if c < 0
                if ~isempty(line), complete = true; end
                break;
            end
            readAny = true;
            if c == 10
                complete = true;
                break;
            end
            line = [line char(c)];
        end
        if ~proc.isAlive() && ~readAny && ~rdr.ready(), break; end
        if ~complete, pause(0.05); end
    end
    if isempty(line) || ~complete
        proc.getOutputStream().close();
        proc.destroy();
        if ~proc.waitFor(2, javaMethod('valueOf', 'java.util.concurrent.TimeUnit', 'SECONDS'))
            proc.destroyForcibly();
            proc.waitFor();
        end
        error('opensysml:transport', 'sysml-grpc reported no address within %ds', timeoutSec);
    end
    conn = opensysml.external(char(line));
    conn.timeout = timeoutSec;
    conn.privateService = true;
    conn.process = proc;
    conn.childStdin = proc.getOutputStream();
end

function tf = isJavaAvailable()
    try
        javaObject('java.lang.ProcessBuilder', javaObject('java.util.ArrayList'));
        tf = true;
    catch
        tf = false;
    end
end
