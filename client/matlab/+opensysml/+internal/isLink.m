function tf = isLink(path)
%ISLINK Whether a path is itself a symbolic link, dangling or not. Read
%   natively under Octave, through Java under MATLAB; false without a JVM.
    tf = false;
    if exist('OCTAVE_VERSION', 'builtin')
        [info, err] = lstat(path);
        tf = err == 0 && S_ISLNK(info.mode);
    elseif usejava('jvm')
        try
            tf = logical(javaMethod('isSymbolicLink', 'java.nio.file.Files', ...
                javaObject('java.io.File', path).toPath()));
        catch
            tf = false;
        end
    end
end
