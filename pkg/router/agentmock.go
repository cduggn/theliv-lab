/*
 * Copyright FMR LLC <opensource@fidelity.com>
 *
 * SPDX-License-Identifier: Apache
 */
package router

import (
	"net/http"

	"github.com/fidelity/theliv/internal/problem"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

var imagePullIssue = &problem.ReportCardIssue{
	Name:        "ContainerWaitingAsImagePullBackOff",
	Description: "Container demo of Pod missing-image-7d9f6c5b4-x2k9p in namespace theliv-demo is waiting due to ImagePullBackOff, reason is ImagePullBackOff",
	CauseLevel:  1,
	Tags: map[string]string{
		"namespace":    "theliv-demo",
		"pod":          "missing-image-7d9f6c5b4-x2k9p",
		"container":    "demo",
		"image":        "busybox:theliv-intentionally-missing",
		"resourcetype": "container",
	},
	Solutions: []string{
		"1. Unable to pull image busybox:theliv-intentionally-missing for the container demo. The root cause could be one of the following.",
		"2. Either the image repository name is incorrect or does NOT exist.",
		"3. Either the image tag theliv-intentionally-missing is invalid or does NOT exist.",
	},
	Commands: []string{
		"kubectl describe po missing-image-7d9f6c5b4-x2k9p -n theliv-demo",
		"kubectl get events --field-selector involvedObject.name=missing-image-7d9f6c5b4-x2k9p -n theliv-demo",
	},
}

var MockReport = []*problem.ReportCard{
	{
		Name:            "missing-image",
		TopResourceType: "Deployment",
		Level:           problem.UserNamespace,
		ID:              "1",
		RootCause:       imagePullIssue,
		Resources: []*problem.ReportCardResource{
			{
				Name:   "missing-image-7d9f6c5b4-x2k9p",
				Type:   "Pod",
				Labels: map[string]string{"app": "missing-image"},
				Issue:  imagePullIssue,
			},
		},
	},
}

func AgentMock(r chi.Router) {
	r.Get("/mock/report", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, MockReport)
	})
}
