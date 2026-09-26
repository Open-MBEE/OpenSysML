function run_tests()
%RUN_TESTS Run every test_*.m here and exit non-zero on failure. Works in
%MATLAB and in GNU Octave (octave --no-gui --eval "run_tests").

    here = fileparts(mfilename('fullpath'));
    addpath(here);
    addpath(fullfile(here, 'private'));
    addpath(fullfile(here, '..'));

    files = {};
    d = dir(fullfile(here, 'test_*.m'));
    for i = 1:numel(d), files{end+1} = d(i).name; end
    files = sort(files);

    passed = 0; failed = 0;
    for i = 1:numel(files)
        name = files{i}(1:end-2);
        try
            feval(name);
            passed = passed + 1;
            fprintf('PASS %s\n', name);
        catch e
            failed = failed + 1;
            fprintf(2, 'FAIL %s: %s (%s)\n', name, e.message, e.identifier);
        end
    end
    fprintf('test files: %d passed, %d failed\n', passed, failed);
    if failed > 0
        exit(1);
    end
end
