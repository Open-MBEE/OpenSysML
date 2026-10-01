function diags = diagnostics(model)
%DIAGNOSTICS The model's parse and validation findings as a cell of structs
%with severity, message, file, line, col, code.

    diags = model.diagnostics;
end
