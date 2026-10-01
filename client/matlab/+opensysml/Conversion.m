classdef Conversion
%CONVERSION A model converted to another representation.

    properties
        content = ''
        fromFormat = ''
        toFormat = ''
        diagnostics = {}
        experimental = false
        experimentalNotice = ''
    end

    methods
        function obj = Conversion(content, fromFormat, toFormat, diagnostics, experimentalNotice)
            if nargin < 1, content = ''; end
            if nargin < 2, fromFormat = ''; end
            if nargin < 3, toFormat = ''; end
            if nargin < 4, diagnostics = {}; end
            if nargin < 5, experimentalNotice = ''; end
            obj.content = content;
            obj.fromFormat = fromFormat;
            obj.toFormat = toFormat;
            obj.diagnostics = diagnostics;
            obj.experimental = opensysml.isExperimental(fromFormat, toFormat);
            obj.experimentalNotice = experimentalNotice;
            if obj.experimental
                if isempty(obj.experimentalNotice)
                    obj.experimentalNotice = ['RDF conversion — Turtle and the API''s JSON element form alike — is ' ...
                        'experimental: the mapping covers model structure and the behavior its bodies state, ' ...
                        'refuses what it cannot write back, and its vocabulary may change without a ' ...
                        'compatibility path; see docs/reference/rdf-mapping.md § Status'];
                end
                warning('opensysml:experimental', '%s', obj.experimentalNotice);
            end
        end

        function text = char(obj)
            text = obj.content;
        end

        function n = length(obj)
            n = length(obj.content);
        end

        function path = write(obj, path)
            fid = fopen(path, 'wb');
            if fid < 0
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('cannot open %s for writing', path));
            end
            cleanup = onCleanup(@() fclose(fid));
            bytes = unicode2native(obj.content, 'UTF-8');
            fwrite(fid, uint8(bytes), 'uint8');
            clear cleanup;
        end
    end
end
