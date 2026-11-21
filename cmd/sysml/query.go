package main

import (
	"fmt"
	"os"
)

func runQuery(files []string, text string) int {
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "sysml: no model to query; name a model file")
		return exitUnevaluable
	}
	sess := newSession()
	status, err := loadFiles(sess, files)
	if err != nil {
		return fail(err)
	}
	if status != exitHolds {
		return status
	}
	var lines []string
	if asViewPath == "" {
		lines, err = sess.Query(text)
	} else {
		var written string
		lines, written, err = sess.QueryAsView(text)
		// A path or model error is an outcome too: its view is written before the error is reported.
		switch {
		case written == "":
		case asViewPath != stdoutPath:
			if werr := writeOutputFile(asViewPath, []byte(written), "QueryResult view"); werr != nil {
				return fail(werr)
			}
		case err != nil:
			writeLines(os.Stdout, resultViewLines(written, false))
		default:
			lines = resultViewLines(written, false)
		}
	}
	if err != nil {
		return fail(err)
	}
	if len(lines) == 0 {
		// Stdout carries one matched element per line, so this report stays out
		// of it.
		fmt.Fprintln(os.Stderr, "sysml: no elements matched")
		return exitHolds
	}
	writeLines(os.Stdout, lines)
	return exitHolds
}
