function result = exportGraphs(model, subject)
%EXPORTGRAPHS Export the lowered graph of an action or state machine.
%   RESULT = EXPORTGRAPHS(MODEL, SUBJECT) answers a struct with fields content
%   (the canonical graphs:1 JSON an external analysis engine is sent, ending
%   in one newline), version (the form's version) and subject (the qualified
%   name as resolved).

    capability = {'export_graphs'};
    model.connection.requireAll(capability);
    request = struct('modelHash', model.hash, 'subject', char(subject));
    answer = opensysml.call(model.connection, 'ExportGraphs', request, capability);
    result = struct('content', '', 'version', 0, 'subject', '');
    if isfield(answer, 'content'), result.content = char(answer.content); end
    if isfield(answer, 'version'), result.version = double(answer.version); end
    if isfield(answer, 'subject'), result.subject = char(answer.subject); end
end
