package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogTimestampsUTC(t *testing.T) {
	var record struct {
		Time string `json:"time"`
	}

	directory := t.TempDir()
	binary := filepath.Join(directory, "quotevendor")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	command := exec.Command(binary, "-config", filepath.Join(directory, "missing.json"))
	command.Env = append(os.Environ(), "TZ=Pacific/Auckland")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("startup unexpectedly accepted a missing config")
	}

	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&record); err != nil {
		t.Fatalf("decode startup log: %v", err)
	}

	if _, err := time.Parse(time.RFC3339Nano, record.Time); err != nil {
		t.Fatalf("invalid log timestamp: %v", err)
	}

	if !strings.HasSuffix(record.Time, "Z") {
		t.Fatalf("log timestamp is not UTC: %s", record.Time)
	}
}
