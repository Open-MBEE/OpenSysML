//go:build sysml_prod || sysml_noprofile

package main

func startPprof() ([]func(), error) { return nil, nil }
