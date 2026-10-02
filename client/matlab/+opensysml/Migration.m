classdef Migration
%MIGRATION A SysML v1 model migrated to SysML v2, with the account of every element.
%   content is the migrated model in toFormat; report is a struct with source,
%   exporter, summary, the mapped, approximated, unmapped and skipped counts,
%   entries (a cell of structs with id, kind, name, target, verdict and note,
%   when asked for) and text (the report, when asked for); results is the JSON
%   index of result snapshots when asked for; files maps the relative path of
%   each image the model's diagrams embed to its uint8 bytes; sourcePath is the
%   v1 file the model came from, absolute, or '' for inline content.

    properties
        content = ''
        fromFormat = ''
        toFormat = ''
        report = struct()
        results = ''
        files = containers.Map('KeyType', 'char', 'ValueType', 'any')
        sourcePath = ''
        experimental = true
        experimentalNotice = ''
    end

    methods
        function obj = Migration(answer, fromFormat, toFormat, sourcePath)
            if nargin < 1, answer = struct(); end
            if nargin < 2, fromFormat = ''; end
            if nargin < 3, toFormat = ''; end
            if nargin < 4, sourcePath = ''; end
            obj.content = char(fieldOr(answer, 'content', ''));
            obj.fromFormat = char(fieldOr(answer, 'fromFormat', fromFormat));
            obj.toFormat = char(fieldOr(answer, 'toFormat', toFormat));
            obj.report = decode_report(fieldOr(answer, 'report', struct()));
            obj.results = char(fieldOr(answer, 'results', ''));
            obj.files = containers.Map('KeyType', 'char', 'ValueType', 'any');
            raw = fieldOr(answer, 'files', {});
            if isstruct(raw), raw = num2cell(raw(:)'); end
            for i = 1:numel(raw)
                file = raw{i};
                obj.files(char(fieldOr(file, 'path', ''))) = ...
                    opensysml.internal.base64Decode(fieldOr(file, 'content', ''));
            end
            obj.sourcePath = char(sourcePath);
            obj.experimentalNotice = char(fieldOr(answer, 'experimentalNotice', ''));
            if isempty(obj.experimentalNotice)
                section = opensysml.internal.unicodeChar(167);
                obj.experimentalNotice = ['SysML v1 migration is experimental: the mapping covers ' ...
                    'structure, ports and connectors, requirements, constraints, instances and ' ...
                    'allocations, reports every element it approximates or leaves behind, and ' ...
                    'what it writes for a v1 element may change without a compatibility path; ' ...
                    'see docs/reference/sysml-v1-migration.md ' section ' Status'];
            end
            warning('opensysml:experimental', '%s', obj.experimentalNotice);
        end

        function text = char(obj)
            text = obj.content;
        end

        function n = length(obj)
            n = length(obj.content);
        end

        function entries = byVerdict(obj, verdict)
        %BYVERDICT The report entries with a verdict: mapped, approximated, unmapped or skipped.
            entries = obj.report.entries(cellfun(@(e) strcmp(e.verdict, char(verdict)), ...
                obj.report.entries));
        end

        function path = write(obj, path)
        %WRITE Write the migrated model to path and its image files beside it, at
        %   their relative paths under path's directory, as sysml -migrate -o writes
        %   them. Nothing is written until every destination is judged: a path naming
        %   the v1 model the migration came from — by any spelling, hard link or
        %   symbolic link — an image that would land outside the model's directory
        %   through .., an absolute path or a symbolic link, or one whose own path
        %   is a symbolic link, raises opensysml:argument. A directory on the way
        %   replaced while the write is under way is not guarded against, as
        %   sysml -migrate -o does not either.
            path = char(path);
            absolute = opensysml.internal.landing(path);
            source = '';
            if ~isempty(obj.sourcePath) && exist(obj.sourcePath, 'file') == 2
                source = opensysml.internal.landing(obj.sourcePath);
            end
            if ~isempty(source) && same_file(absolute, source)
                opensysml.internal.raise('opensysml:argument', sprintf( ...
                    '%s names the model being migrated; the v1 model would be replaced by its migration', path));
            end
            base = fileparts(absolute);
            names = sort(keys(obj.files));
            destinations = cell(1, numel(names));
            for i = 1:numel(names)
                name = names{i};
                segments = strsplit(name, '/', 'CollapseDelimiters', false);
                malformed = any(cellfun(@(s) any(strcmp(s, {'', '.', '..'})) || ...
                    any(s == '\'), segments));
                file = fullfile(base, segments{:});
                landed = '';
                if ~malformed && within(file, base)
                    landed = opensysml.internal.landing(file);
                end
                if isempty(landed) || ~within(landed, base)
                    opensysml.internal.raise('opensysml:argument', sprintf( ...
                        'the migration''s image %s would land outside %s', name, base));
                end
                if same_file(landed, absolute) || (~isempty(source) && same_file(landed, source))
                    opensysml.internal.raise('opensysml:argument', sprintf( ...
                        'the migration''s image %s would replace %s', name, file));
                end
                if opensysml.internal.isLink(file)
                    opensysml.internal.raise('opensysml:argument', sprintf( ...
                        'the migration''s image %s would be written through a symbolic link at %s', name, file));
                end
                destinations{i} = file;
            end
            write_bytes(path, unicode2native(obj.content, 'UTF-8'));
            for i = 1:numel(names)
                directory = fileparts(destinations{i});
                if ~isempty(directory) && exist(directory, 'dir') ~= 7
                    [ok, message] = mkdir(directory);
                    if ~ok
                        opensysml.internal.raise('opensysml:argument', ...
                            sprintf('cannot create %s: %s', directory, message));
                    end
                end
                write_bytes(destinations{i}, obj.files(names{i}));
            end
        end
    end
end

function report = decode_report(raw)
    report = struct('source', char(fieldOr(raw, 'source', '')), ...
        'exporter', char(fieldOr(raw, 'exporter', '')), ...
        'summary', char(fieldOr(raw, 'summary', '')), ...
        'mapped', double(fieldOr(raw, 'mapped', 0)), ...
        'approximated', double(fieldOr(raw, 'approximated', 0)), ...
        'unmapped', double(fieldOr(raw, 'unmapped', 0)), ...
        'skipped', double(fieldOr(raw, 'skipped', 0)), ...
        'entries', {{}}, 'text', char(fieldOr(raw, 'text', '')));
    entries = fieldOr(raw, 'entries', {});
    if isstruct(entries), entries = num2cell(entries(:)'); end
    decoded = cell(1, numel(entries));
    for i = 1:numel(entries)
        e = entries{i};
        decoded{i} = struct('id', char(fieldOr(e, 'id', '')), 'kind', char(fieldOr(e, 'kind', '')), ...
            'name', char(fieldOr(e, 'name', '')), 'target', char(fieldOr(e, 'target', '')), ...
            'verdict', char(fieldOr(e, 'verdict', '')), 'note', char(fieldOr(e, 'note', '')));
    end
    report.entries = decoded;
end

function tf = same_file(a, b)
%SAME_FILE Whether two landed paths name one file: the same spelling, or the
%   same inode when both exist.
    if ispc
        tf = strcmpi(a, b);
    else
        tf = strcmp(a, b);
    end
    tf = tf || opensysml.internal.sameFile(a, b);
end

function tf = within(path, base)
%WITHIN Whether a path lies strictly under a directory, the root included.
    if ~isempty(base) && base(end) ~= filesep
        base = [base filesep];
    end
    tf = numel(path) > numel(base) && strncmp(path, base, numel(base));
end

function write_bytes(path, bytes)
    fid = fopen(path, 'wb');
    if fid < 0
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('cannot open %s for writing', path));
    end
    cleanup = onCleanup(@() fclose(fid));
    fwrite(fid, uint8(bytes), 'uint8');
    clear cleanup;
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name); else, value = fallback; end
end
