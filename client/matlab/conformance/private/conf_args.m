function opts = conf_args(args)
%CONF_ARGS The runner's flags, in the single- and double-dash spellings:
%--binary/--address/--scenarios/--fixtures/--run/--report take a value;
%--allow-skips and -v are switches; -h/--help prints usage.

    opts.binary = ''; opts.address = '';
    opts.scenarios = ''; opts.fixtures = '';
    opts.run = ''; opts.report = '';
    opts.allow_skips = false; opts.verbose = false;
    i = 1;
    takes_value = {'--binary', '-binary', '--address', '-address', ...
                   '--scenarios', '-scenarios', '--fixtures', '-fixtures', ...
                   '--run', '-run', '--report', '-report'};
    while i <= numel(args)
        arg = args{i};
        if ismember(arg, takes_value)
            i = i + 1;
            if i > numel(args), error('opensysml:conformance', '%s needs a value', arg); end
            key = strrep(regexprep(arg, '^-+', ''), '-', '_');
            opts.(key) = args{i};
        elseif ismember(arg, {'--allow-skips', '-allow-skips'})
            opts.allow_skips = true;
        elseif ismember(arg, {'-v', '--verbose'})
            opts.verbose = true;
        elseif ismember(arg, {'-h', '--help'})
            disp('run_conformance [--binary PATH | --address HOST:PORT] [--scenarios DIR] [--fixtures DIR] [--run SUBSTRING] [--report FILE|-] [--allow-skips] [-v]');
            exit(0);
        else
            error('opensysml:conformance', 'unknown flag "%s"', arg);
        end
        i = i + 1;
    end
end
