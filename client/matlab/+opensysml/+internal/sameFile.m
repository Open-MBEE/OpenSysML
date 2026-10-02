function tf = sameFile(a, b)
%SAMEFILE Whether two existing paths name one file — the same inode through a
%   hard link, a symbolic link or another spelling. False when either is
%   absent, or without a JVM under MATLAB, where identity cannot be read.
    tf = false;
    if exist('OCTAVE_VERSION', 'builtin')
        [infoA, errA] = stat(a);
        [infoB, errB] = stat(b);
        tf = errA == 0 && errB == 0 && infoA.ino == infoB.ino && infoA.dev == infoB.dev;
    elseif usejava('jvm')
        try
            tf = logical(javaMethod('isSameFile', 'java.nio.file.Files', ...
                javaObject('java.io.File', a).toPath(), javaObject('java.io.File', b).toPath()));
        catch
            tf = false;
        end
    end
end
