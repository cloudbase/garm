//go:build integration
// +build integration

// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v71/github"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func (suite *GarmSuite) TestWorkflowJobs() {
	suite.TriggerWorkflow(suite.ghToken, orgName, repoName, workflowFileName, "org-runner")
	suite.ValidateJobLifecycle("org-runner")

	suite.TriggerWorkflow(suite.ghToken, orgName, repoName, workflowFileName, "repo-runner")
	suite.ValidateJobLifecycle("repo-runner")
}

func (suite *GarmSuite) TriggerWorkflow(ghToken, orgName, repoName, workflowFileName, labelName string) {
	t := suite.T()
	t.Logf("Trigger workflow with label %s", labelName)

	client := getGithubClient(ghToken)
	eventReq := github.CreateWorkflowDispatchEventRequest{
		Ref: "main",
		Inputs: map[string]interface{}{
			"sleep_time":   "50",
			"runner_label": labelName,
		},
	}
	_, err := client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), orgName, repoName, workflowFileName, eventReq)
	suite.NoError(err, "error triggering workflow")
}

func (suite *GarmSuite) ValidateJobLifecycle(label string) {
	t := suite.T()
	t.Logf("Validate GARM job lifecycle with label %s", label)

	// wait for job list to be updated
	job, err := suite.waitLabelledJob(label, 4*time.Minute)
	suite.NoError(err, "error waiting for job to be created")

	// check expected job status
	job, err = suite.waitJobStatus(job.ID, params.JobStatusQueued, 4*time.Minute)
	suite.NoError(err, "error waiting for job to be queued")

	job, err = suite.waitJobStatus(job.ID, params.JobStatusInProgress, 4*time.Minute)
	suite.NoError(err, "error waiting for job to be in progress")

	// check expected instance status
	instance, err := suite.waitInstanceStatus(job.RunnerName, commonParams.InstanceRunning, params.RunnerActive, 5*time.Minute)
	suite.NoError(err, "error waiting for instance to be running")

	// wait for job to be completed
	_, err = suite.waitJobStatus(job.ID, params.JobStatusCompleted, 4*time.Minute)
	suite.NoError(err, "error waiting for job to be completed")

	// wait for instance to be removed
	err = suite.WaitInstanceToBeRemoved(instance.Name, 5*time.Minute)
	suite.NoError(err, "error waiting for instance to be removed")

	// wait for GARM to rebuild the pool running idle instances
	err = suite.WaitPoolInstances(instance.PoolID, commonParams.InstanceRunning, params.RunnerIdle, 5*time.Minute)
	suite.NoError(err, "error waiting for pool instances to be running idle")
}

func (suite *GarmSuite) waitLabelledJob(label string, timeout time.Duration) (*params.Job, error) {
	t := suite.T()
	var timeWaited time.Duration // default is 0
	var jobs params.Jobs
	var err error

	t.Logf("Waiting for job with label %s", label)
	for timeWaited < timeout {
		jobs, err = listJobs(suite.cli, suite.authToken)
		if err != nil {
			return nil, err
		}
		for _, job := range jobs {
			for _, jobLabel := range job.Labels {
				if jobLabel == label {
					return &job, err
				}
			}
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	if err := printJSONResponse(jobs); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("failed to wait job with label %s", label)
}

func (suite *GarmSuite) waitJobStatus(id int64, status params.JobStatus, timeout time.Duration) (*params.Job, error) {
	t := suite.T()
	var timeWaited time.Duration // default is 0
	var job *params.Job

	t.Logf("Waiting for job %d to reach status %v", id, status)
	for timeWaited < timeout {
		jobs, err := listJobs(suite.cli, suite.authToken)
		if err != nil {
			return nil, err
		}

		job = nil
		for k, v := range jobs {
			if v.ID == id {
				job = &jobs[k]
				break
			}
		}

		if job == nil {
			if status == params.JobStatusCompleted {
				// The job is not found in the list. We can safely assume
				// that it is completed
				return nil, nil
			}
			// if the job is not found, and expected status is not "completed",
			// we need to error out.
			return nil, fmt.Errorf("job %d not found, expected to be found in status %s", id, status)
		} else if job.Status == string(status) {
			return job, nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	if err := printJSONResponse(*job); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("timeout waiting for job %d to reach status %s", id, status)
}

func (suite *GarmSuite) waitInstanceStatus(name string, status commonParams.InstanceStatus, runnerStatus params.RunnerStatus, timeout time.Duration) (*params.Instance, error) {
	t := suite.T()
	var timeWaited time.Duration // default is 0
	var instance *params.Instance
	var err error

	t.Logf("Waiting for instance %s to reach desired status %v and desired runner status %v", name, status, runnerStatus)
	for timeWaited < timeout {
		instance, err = getInstance(suite.cli, suite.authToken, name)
		if err != nil {
			return nil, err
		}
		t.Logf("Instance %s has status %v and runner status %v", name, instance.Status, instance.RunnerStatus)
		if instance.Status == status && instance.RunnerStatus == runnerStatus {
			return instance, nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	if err := printJSONResponse(*instance); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("timeout waiting for instance %s status to reach status %s and runner status %s", name, status, runnerStatus)
}
