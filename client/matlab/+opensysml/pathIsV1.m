function tf = pathIsV1(path)
%PATHISV1 Whether a path's extension names a SysML v1 model: .xmi, .uml or .mdzip.

    [~, ~, extension] = fileparts(char(path));
    tf = ~isempty(extension) && opensysml.isV1(extension(2:end));
end
