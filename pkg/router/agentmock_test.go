/*
 * Copyright FMR LLC <opensource@fidelity.com>
 *
 * SPDX-License-Identifier: Apache
 */
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fidelity/theliv/internal/problem"
	"github.com/go-chi/chi/v5"
)

func TestAgentMockReport(t *testing.T) {
	r := chi.NewRouter()
	r.Route("/agent", AgentMock)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agent/mock/report", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var cards []problem.ReportCard
	if err := json.Unmarshal(rec.Body.Bytes(), &cards); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards) = %d, want 1", len(cards))
	}
	c := cards[0]
	if c.RootCause == nil || c.RootCause.Name != "ContainerWaitingAsImagePullBackOff" {
		t.Fatalf("rootCause = %+v", c.RootCause)
	}
	if len(c.Resources) != 1 || c.Resources[0].Issue == nil {
		t.Fatalf("resources = %+v", c.Resources)
	}
	issue := c.Resources[0].Issue
	wantSol := "1. Unable to pull image busybox:theliv-intentionally-missing for the container demo. The root cause could be one of the following."
	if issue.Solutions[0] != wantSol {
		t.Errorf("solutions[0] = %q", issue.Solutions[0])
	}
	if len(issue.Commands) != 2 {
		t.Errorf("len(commands) = %d, want 2", len(issue.Commands))
	}
}
