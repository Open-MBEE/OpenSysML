function requireQuestion(conn, question)
%REQUIREQUESTION Gate verification questions beyond evaluate.

    if isempty(question) || strcmp(question, 'evaluate'), return; end
    conn.require('verification_questions');
end
