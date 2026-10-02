function path = landing(path)
%LANDING Where a path lands once the symbolic links on the way are followed:
%   its longest existing prefix in that prefix's real spelling, a dangling
%   link followed to where it points, and the missing remainder appended as
%   spelled. Under Octave the links are read natively; under MATLAB through
%   Java, or not at all without a JVM, where the spelling is the answer.
    head = opensysml.internal.absolutePath(path);
    tail = {};
    hops = 0;
    while true
        real = real_path(head);
        if ~isempty(real)
            path = fullfile(real, tail{:});
            return;
        end
        target = read_link(head);
        if ~isempty(target)
            hops = hops + 1;
            if hops >= 40
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('%s: too many levels of symbolic links', path));
            end
            if is_absolute(target)
                head = opensysml.internal.absolutePath(target);
            else
                head = opensysml.internal.absolutePath(fullfile(fileparts(head), target));
            end
            continue;
        end
        [parent, name, ext] = fileparts(head);
        if isempty(parent) || strcmp(parent, head)
            path = fullfile(head, tail{:});
            return;
        end
        tail = [{[name ext]}, tail];
        head = parent;
    end
end

function real = real_path(path)
    real = '';
    if exist('OCTAVE_VERSION', 'builtin')
        [real, err] = canonicalize_file_name(path);
        if err ~= 0, real = ''; end
    elseif usejava('jvm')
        try
            file = javaObject('java.io.File', path);
            real = char(file.toPath().toRealPath(javaArray('java.nio.file.LinkOption', 0)).toString());
        catch
            real = '';
        end
    elseif exist(path, 'file') ~= 0
        real = path;
    end
end

function target = read_link(path)
    target = '';
    if exist('OCTAVE_VERSION', 'builtin')
        [target, err] = readlink(path);
        if err ~= 0, target = ''; end
    elseif usejava('jvm')
        try
            linked = javaObject('java.io.File', path).toPath();
            if javaMethod('isSymbolicLink', 'java.nio.file.Files', linked)
                target = char(javaMethod('readSymbolicLink', 'java.nio.file.Files', linked).toString());
            end
        catch
            target = '';
        end
    end
end

function tf = is_absolute(path)
    tf = ~isempty(path) && (path(1) == '/' || path(1) == '\' || ...
        (numel(path) >= 2 && path(2) == ':'));
end
