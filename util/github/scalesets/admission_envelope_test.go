// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.
package scalesets

import (
	"encoding/json"
	"testing"

	"github.com/cloudbase/garm/params"
)

func TestAdmissionMessageEnvelope(t *testing.T) {
	complete := `{"totalAcquiredJobs":0,"totalAssignedJobs":0,"totalRunningJobs":0,"totalRegisteredRunners":0,"totalBusyRunners":0,"totalIdleRunners":0}`
	tests := []struct {
		name, raw string
		valid     bool
	}{
		{"zero message ID", `{"messageId":0,"messageType":"jobs","body":"[]","statistics":` + complete + `}`, true},
		{"missing ID", `{"messageType":"jobs","body":"[]","statistics":` + complete + `}`, false},
		{"missing counters", `{"messageId":1,"messageType":"jobs","body":"[]","statistics":{}}`, false},
		{"null counters", `{"messageId":1,"messageType":"jobs","body":"[]","statistics":null}`, false},
		{"negative ID", `{"messageId":-1,"messageType":"jobs","body":"[]","statistics":` + complete + `}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg params.RunnerScaleSetMessage
			if err := json.Unmarshal([]byte(tt.raw), &msg); err != nil {
				t.Fatal(err)
			}
			if got := validAdmissionMessage([]byte(tt.raw), msg); got != tt.valid {
				t.Fatalf("valid=%v", got)
			}
		})
	}
	if admissionStatisticsComplete(json.RawMessage(`{}`)) {
		t.Fatal("missing initial counters accepted")
	}
	if !admissionStatisticsComplete(json.RawMessage(complete)) {
		t.Fatal("explicit initial zero counters rejected")
	}
}
