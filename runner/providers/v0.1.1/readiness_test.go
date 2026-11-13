// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package v011

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/runner/common"
)

func TestReadinessCommand(t *testing.T) {
	dir := t.TempDir()
	request := common.ReadinessRequest{Version: 1, RequestID: "nonce", Generation: "generation", Domain: "domain"}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "request"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(common.ReadinessResponse{Request: request, Ready: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "response"), response, 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$GARM_COMMAND\" = readiness_v1 ] || exit 1\n[ \"$GARM_CONTROLLER_ID\" = controller ] || exit 1\ncmp -s - \"$FIXTURE_DIR/request\" || exit 1\ncat \"$FIXTURE_DIR/response\"\n"
	bin := filepath.Join(dir, "provider")
	// The temporary provider fixture must be executable by this test process.
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { // #nosec G306 -- test-only executable
		t.Fatal(err)
	}
	e := external{execPath: bin, controllerID: "controller", cfg: &config.Provider{}, environmentVariables: []string{"FIXTURE_DIR=" + dir, "PATH=" + os.Getenv("PATH")}}
	got, err := e.Readiness(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got.Request != request || !got.Ready {
		t.Fatalf("unexpected response: %+v", got)
	}
	for _, invalid := range []string{`{"unknown":true}`, `{ } { }`} {
		if err := os.WriteFile(filepath.Join(dir, "response"), []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Readiness(context.Background(), request); err == nil {
			t.Fatalf("accepted invalid response %s", invalid)
		}
	}
}
