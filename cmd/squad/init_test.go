package main

import (
	"bytes"
	"testing"
)

func TestInitCmd_HasYesAndDirFlags(t *testing.T) {
	cmd := newInitCmd()
	if cmd.Flags().Lookup("yes") == nil {
		t.Fatal("missing --yes flag")
	}
	if cmd.Flags().Lookup("dir") == nil {
		t.Fatal("missing --dir flag")
	}
}

func TestInitCmd_ShortMentionsScaffold(t *testing.T) {
	cmd := newInitCmd()
	if !bytes.Contains([]byte(cmd.Short+cmd.Long), []byte("scaffold")) &&
		!bytes.Contains([]byte(cmd.Short+cmd.Long), []byte("Scaffold")) {
		t.Fatalf("Short/Long should mention scaffold; Short=%q Long=%q", cmd.Short, cmd.Long)
	}
}
