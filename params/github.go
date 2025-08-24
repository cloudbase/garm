// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package params

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Event string

const (
	// WorkflowJobEvent is the event set in the webhook payload from github
	// when a workflow_job hook is sent.
	WorkflowJobEvent Event = "workflow_job"
	PingEvent        Event = "ping"
)

// WorkflowJob holds the payload sent by github when a workload_job is sent.
type WorkflowJob struct {
	Action      string `json:"action"`
	WorkflowJob struct {
		ID          int64     `json:"id"`
		RunID       int64     `json:"run_id"`
		RunURL      string    `json:"run_url"`
		RunAttempt  int64     `json:"run_attempt"`
		NodeID      string    `json:"node_id"`
		HeadSha     string    `json:"head_sha"`
		URL         string    `json:"url"`
		HTMLURL     string    `json:"html_url"`
		Status      string    `json:"status"`
		Conclusion  string    `json:"conclusion"`
		StartedAt   time.Time `json:"started_at"`
		CompletedAt time.Time `json:"completed_at"`
		Name        string    `json:"name"`
		Steps       []struct {
			Name        string    `json:"name"`
			Status      string    `json:"status"`
			Conclusion  string    `json:"conclusion"`
			Number      int64     `json:"number"`
			StartedAt   time.Time `json:"started_at"`
			CompletedAt time.Time `json:"completed_at"`
		} `json:"steps"`
		CheckRunURL     string   `json:"check_run_url"`
		Labels          []string `json:"labels"`
		RunnerID        int64    `json:"runner_id"`
		RunnerName      string   `json:"runner_name"`
		RunnerGroupID   int64    `json:"runner_group_id"`
		RunnerGroupName string   `json:"runner_group_name"`
	} `json:"workflow_job"`
	Repository struct {
		ID       int64  `json:"id"`
		NodeID   string `json:"node_id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
		Owner    struct {
			Login             string `json:"login"`
			ID                int64  `json:"id"`
			NodeID            string `json:"node_id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"owner"`
		HTMLURL          string    `json:"html_url"`
		Description      string    `json:"description"`
		Fork             bool      `json:"fork"`
		URL              string    `json:"url"`
		ForksURL         string    `json:"forks_url"`
		KeysURL          string    `json:"keys_url"`
		CollaboratorsURL string    `json:"collaborators_url"`
		TeamsURL         string    `json:"teams_url"`
		HooksURL         string    `json:"hooks_url"`
		IssueEventsURL   string    `json:"issue_events_url"`
		EventsURL        string    `json:"events_url"`
		AssigneesURL     string    `json:"assignees_url"`
		BranchesURL      string    `json:"branches_url"`
		TagsURL          string    `json:"tags_url"`
		BlobsURL         string    `json:"blobs_url"`
		GitTagsURL       string    `json:"git_tags_url"`
		GitRefsURL       string    `json:"git_refs_url"`
		TreesURL         string    `json:"trees_url"`
		StatusesURL      string    `json:"statuses_url"`
		LanguagesURL     string    `json:"languages_url"`
		StargazersURL    string    `json:"stargazers_url"`
		ContributorsURL  string    `json:"contributors_url"`
		SubscribersURL   string    `json:"subscribers_url"`
		SubscriptionURL  string    `json:"subscription_url"`
		CommitsURL       string    `json:"commits_url"`
		GitCommitsURL    string    `json:"git_commits_url"`
		CommentsURL      string    `json:"comments_url"`
		IssueCommentURL  string    `json:"issue_comment_url"`
		ContentsURL      string    `json:"contents_url"`
		CompareURL       string    `json:"compare_url"`
		MergesURL        string    `json:"merges_url"`
		ArchiveURL       string    `json:"archive_url"`
		DownloadsURL     string    `json:"downloads_url"`
		IssuesURL        string    `json:"issues_url"`
		PullsURL         string    `json:"pulls_url"`
		MilestonesURL    string    `json:"milestones_url"`
		NotificationsURL string    `json:"notifications_url"`
		LabelsURL        string    `json:"labels_url"`
		ReleasesURL      string    `json:"releases_url"`
		DeploymentsURL   string    `json:"deployments_url"`
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
		PushedAt         time.Time `json:"pushed_at"`
		GitURL           string    `json:"git_url"`
		SSHURL           string    `json:"ssh_url"`
		CloneURL         string    `json:"clone_url"`
		SvnURL           string    `json:"svn_url"`
		Homepage         *string   `json:"homepage"`
		Size             int64     `json:"size"`
		StargazersCount  int64     `json:"stargazers_count"`
		WatchersCount    int64     `json:"watchers_count"`
		Language         *string   `json:"language"`
		HasIssues        bool      `json:"has_issues"`
		HasProjects      bool      `json:"has_projects"`
		HasDownloads     bool      `json:"has_downloads"`
		HasWiki          bool      `json:"has_wiki"`
		HasPages         bool      `json:"has_pages"`
		ForksCount       int64     `json:"forks_count"`
		MirrorURL        *string   `json:"mirror_url"`
		Archived         bool      `json:"archived"`
		Disabled         bool      `json:"disabled"`
		OpenIssuesCount  int64     `json:"open_issues_count"`
		License          struct {
			Key    string `json:"key"`
			Name   string `json:"name"`
			SpdxID string `json:"spdx_id"`
			URL    string `json:"url"`
			NodeID string `json:"node_id"`
		} `json:"license"`
		AllowForking bool `json:"allow_forking"`
		IsTemplate   bool `json:"is_template"`
		// Topics        []interface{} `json:"topics"`
		Visibility    string `json:"visibility"`
		Forks         int64  `json:"forks"`
		OpenIssues    int64  `json:"open_issues"`
		Watchers      int64  `json:"watchers"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
	Organization struct {
		Login string `json:"login"`
		// Name is a gitea specific field
		Name             string `json:"name"`
		ID               int64  `json:"id"`
		NodeID           string `json:"node_id"`
		URL              string `json:"url"`
		ReposURL         string `json:"repos_url"`
		EventsURL        string `json:"events_url"`
		HooksURL         string `json:"hooks_url"`
		IssuesURL        string `json:"issues_url"`
		MembersURL       string `json:"members_url"`
		PublicMembersURL string `json:"public_members_url"`
		AvatarURL        string `json:"avatar_url"`
		Description      string `json:"description"`
	} `json:"organization"`
	Enterprise struct {
		ID        int64  `json:"id"`
		Slug      string `json:"slug"`
		Name      string `json:"name"`
		NodeID    string `json:"node_id"`
		AvatarURL string `json:"avatar_url"`
		// Description interface{} `json:"description"`
		// WebsiteURL  interface{} `json:"website_url"`
		HTMLURL   string    `json:"html_url"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"enterprise"`
	Sender struct {
		Login             string `json:"login"`
		ID                int64  `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"sender"`
}

func (w WorkflowJob) GetOrgName(forgeType EndpointType) string {
	if forgeType == GiteaEndpointType {
		return w.Organization.Name
	}
	return w.Organization.Login
}

type RunnerSetting struct {
	Ephemeral     bool `json:"ephemeral,omitempty"`
	IsElastic     bool `json:"isElastic,omitempty"`
	DisableUpdate bool `json:"disableUpdate,omitempty"`
}

type Label struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type RunnerScaleSetStatistic struct {
	TotalAvailableJobs     int `json:"totalAvailableJobs"`
	TotalAcquiredJobs      int `json:"totalAcquiredJobs"`
	TotalAssignedJobs      int `json:"totalAssignedJobs"`
	TotalRunningJobs       int `json:"totalRunningJobs"`
	TotalRegisteredRunners int `json:"totalRegisteredRunners"`
	TotalBusyRunners       int `json:"totalBusyRunners"`
	TotalIdleRunners       int `json:"totalIdleRunners"`
}

type RunnerScaleSet struct {
	ID                   int                      `json:"id,omitempty"`
	Name                 string                   `json:"name,omitempty"`
	RunnerGroupID        int64                    `json:"runnerGroupId,omitempty"`
	RunnerGroupName      string                   `json:"runnerGroupName,omitempty"`
	Labels               []Label                  `json:"labels,omitempty"`
	RunnerSetting        RunnerSetting            `json:"RunnerSetting,omitempty"`
	CreatedOn            time.Time                `json:"createdOn,omitempty"`
	RunnerJitConfigURL   string                   `json:"runnerJitConfigUrl,omitempty"`
	GetAcquirableJobsURL string                   `json:"getAcquirableJobsUrl,omitempty"`
	AcquireJobsURL       string                   `json:"acquireJobsUrl,omitempty"`
	Statistics           *RunnerScaleSetStatistic `json:"statistics,omitempty"`
	Status               interface{}              `json:"status,omitempty"`
	Enabled              *bool                    `json:"enabled,omitempty"`
}

type RunnerScaleSetsResponse struct {
	Count           int              `json:"count"`
	RunnerScaleSets []RunnerScaleSet `json:"value"`
}

type ActionsServiceAdminInfoResponse struct {
	URL   string `json:"url,omitempty"`
	Token string `json:"token,omitempty"`
}

func (a ActionsServiceAdminInfoResponse) GetURL() (*url.URL, error) {
	if a.URL == "" {
		return nil, fmt.Errorf("no url specified")
	}
	u, err := url.ParseRequestURI(a.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	return u, nil
}

func (a ActionsServiceAdminInfoResponse) getJWT() (*jwt.Token, error) {
	// We're parsing a token we got from the GitHub API. We can't verify its signature.
	// We do need the expiration date however, or other info.
	token, _, err := jwt.NewParser().ParseUnverified(a.Token, &jwt.RegisteredClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwt token: %w", err)
	}
	return token, nil
}

func (a ActionsServiceAdminInfoResponse) ExiresAt() (time.Time, error) {
	jwt, err := a.getJWT()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode jwt token: %w", err)
	}
	expiration, err := jwt.Claims.GetExpirationTime()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get expiration time: %w", err)
	}

	return expiration.Time, nil
}

func (a ActionsServiceAdminInfoResponse) IsExpired() bool {
	if exp, err := a.ExiresAt(); err == nil {
		return time.Now().UTC().After(exp)
	}
	return true
}

func (a ActionsServiceAdminInfoResponse) TimeRemaining() (time.Duration, error) {
	exp, err := a.ExiresAt()
	if err != nil {
		return 0, fmt.Errorf("failed to get expiration: %w", err)
	}
	now := time.Now().UTC()
	return exp.Sub(now), nil
}

func (a ActionsServiceAdminInfoResponse) ExpiresIn(t time.Duration) bool {
	remaining, err := a.TimeRemaining()
	if err != nil {
		return true
	}
	return remaining <= t
}

type ActionsServiceAdminInfoRequest struct {
	URL         string `json:"url,omitempty"`
	RunnerEvent string `json:"runner_event,omitempty"`
}

type RunnerScaleSetSession struct {
	SessionID               *uuid.UUID               `json:"sessionId,omitempty"`
	OwnerName               string                   `json:"ownerName,omitempty"`
	RunnerScaleSet          *RunnerScaleSet          `json:"runnerScaleSet,omitempty"`
	MessageQueueURL         string                   `json:"messageQueueUrl,omitempty"`
	MessageQueueAccessToken string                   `json:"messageQueueAccessToken,omitempty"`
	Statistics              *RunnerScaleSetStatistic `json:"statistics,omitempty"`
}

func (a RunnerScaleSetSession) GetURL() (*url.URL, error) {
	if a.MessageQueueURL == "" {
		return nil, fmt.Errorf("no url specified")
	}
	u, err := url.ParseRequestURI(a.MessageQueueURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	return u, nil
}

func (a RunnerScaleSetSession) getJWT() (*jwt.Token, error) {
	// We're parsing a token we got from the GitHub API. We can't verify its signature.
	// We do need the expiration date however, or other info.
	token, _, err := jwt.NewParser().ParseUnverified(a.MessageQueueAccessToken, &jwt.RegisteredClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwt token: %w", err)
	}
	return token, nil
}

func (a RunnerScaleSetSession) ExiresAt() (time.Time, error) {
	jwt, err := a.getJWT()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode jwt token: %w", err)
	}
	expiration, err := jwt.Claims.GetExpirationTime()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get expiration time: %w", err)
	}

	return expiration.Time, nil
}

func (a RunnerScaleSetSession) IsExpired() bool {
	if exp, err := a.ExiresAt(); err == nil {
		return time.Now().UTC().After(exp)
	}
	return true
}

func (a RunnerScaleSetSession) TimeRemaining() (time.Duration, error) {
	exp, err := a.ExiresAt()
	if err != nil {
		return 0, fmt.Errorf("failed to get expiration: %w", err)
	}
	now := time.Now().UTC()
	return exp.Sub(now), nil
}

func (a RunnerScaleSetSession) ExpiresIn(t time.Duration) bool {
	remaining, err := a.TimeRemaining()
	if err != nil {
		return true
	}
	return remaining <= t
}

type RunnerScaleSetMessage struct {
	MessageID   int64                    `json:"messageId"`
	MessageType string                   `json:"messageType"`
	Body        string                   `json:"body"`
	Statistics  *RunnerScaleSetStatistic `json:"statistics"`
}

func (r RunnerScaleSetMessage) IsNil() bool {
	return r.MessageID == 0 && r.MessageType == "" && r.Body == "" && r.Statistics == nil
}

func (r RunnerScaleSetMessage) GetJobsFromBody() ([]ScaleSetJobMessage, error) {
	var body []ScaleSetJobMessage
	if r.Body == "" {
		return nil, fmt.Errorf("no body specified")
	}
	if err := json.Unmarshal([]byte(r.Body), &body); err != nil {
		return nil, fmt.Errorf("failed to unmarshal body: %w", err)
	}
	return body, nil
}

type RunnerReference struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	OS                string  `json:"os"`
	RunnerScaleSetID  int     `json:"runnerScaleSetId"`
	CreatedOn         any     `json:"createdOn"`
	RunnerGroupID     uint64  `json:"runnerGroupId"`
	RunnerGroupName   string  `json:"runnerGroupName"`
	Version           string  `json:"version"`
	Enabled           bool    `json:"enabled"`
	Ephemeral         bool    `json:"ephemeral"`
	Status            any     `json:"status"`
	DisableUpdate     bool    `json:"disableUpdate"`
	ProvisioningState string  `json:"provisioningState"`
	Busy              bool    `json:"busy"`
	Labels            []Label `json:"labels,omitempty"`
}

func (r RunnerReference) GetStatus() RunnerStatus {
	status, ok := r.Status.(string)
	if !ok {
		return RunnerUnknown
	}
	runnerStatus := RunnerStatus(status)
	if !runnerStatus.IsValid() {
		return RunnerUnknown
	}

	if runnerStatus == RunnerOnline {
		if r.Busy {
			return RunnerActive
		}
		return RunnerIdle
	}
	return runnerStatus
}

type RunnerScaleSetJitRunnerConfig struct {
	Runner           *RunnerReference `json:"runner"`
	EncodedJITConfig string           `json:"encodedJITConfig"`
}

func (r RunnerScaleSetJitRunnerConfig) DecodedJITConfig() (map[string]string, error) {
	if r.EncodedJITConfig == "" {
		return nil, fmt.Errorf("no encoded JIT config specified")
	}
	decoded, err := base64.StdEncoding.DecodeString(r.EncodedJITConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JIT config: %w", err)
	}
	jitConfig := make(map[string]string)
	if err := json.Unmarshal(decoded, &jitConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JIT config: %w", err)
	}
	return jitConfig, nil
}

type RunnerReferenceList struct {
	Count            int               `json:"count"`
	RunnerReferences []RunnerReference `json:"value"`
}

type AcquirableJobList struct {
	Count int             `json:"count"`
	Jobs  []AcquirableJob `json:"value"`
}

type AcquirableJob struct {
	AcquireJobURL   string   `json:"acquireJobUrl"`
	MessageType     string   `json:"messageType"`
	RunnerRequestID int64    `json:"run0ne00rRequestId"`
	RepositoryName  string   `json:"repositoryName"`
	OwnerName       string   `json:"ownerName"`
	JobWorkflowRef  string   `json:"jobWorkflowRef"`
	EventName       string   `json:"eventName"`
	RequestLabels   []string `json:"requestLabels"`
}

type RunnerGroup struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	IsDefault bool   `json:"isDefaultGroup"`
}

type RunnerGroupList struct {
	Count        int           `json:"count"`
	RunnerGroups []RunnerGroup `json:"value"`
}

type ScaleSetJobMessage struct {
	MessageType        string    `json:"messageType,omitempty"`
	JobID              string    `json:"jobId,omitempty"`
	RunnerRequestID    int64     `json:"runnerRequestId,omitempty"`
	RepositoryName     string    `json:"repositoryName,omitempty"`
	OwnerName          string    `json:"ownerName,omitempty"`
	JobWorkflowRef     string    `json:"jobWorkflowRef,omitempty"`
	JobDisplayName     string    `json:"jobDisplayName,omitempty"`
	WorkflowRunID      int64     `json:"workflowRunId,omitempty"`
	EventName          string    `json:"eventName,omitempty"`
	RequestLabels      []string  `json:"requestLabels,omitempty"`
	QueueTime          time.Time `json:"queueTime,omitempty"`
	ScaleSetAssignTime time.Time `json:"scaleSetAssignTime,omitempty"`
	RunnerAssignTime   time.Time `json:"runnerAssignTime,omitempty"`
	FinishTime         time.Time `json:"finishTime,omitempty"`
	Result             string    `json:"result,omitempty"`
	RunnerID           int64     `json:"runnerId,omitempty"`
	RunnerName         string    `json:"runnerName,omitempty"`
	AcquireJobURL      string    `json:"acquireJobUrl,omitempty"`
}

func (s ScaleSetJobMessage) MessageTypeToStatus() JobStatus {
	switch s.MessageType {
	case MessageTypeJobAssigned:
		return JobStatusQueued
	case MessageTypeJobStarted:
		return JobStatusInProgress
	case MessageTypeJobCompleted:
		return JobStatusCompleted
	default:
		return JobStatusQueued
	}
}

func (s ScaleSetJobMessage) ToJob() Job {
	return Job{
		ScaleSetJobID:   s.JobID,
		Action:          s.EventName,
		RunID:           s.WorkflowRunID,
		Status:          string(s.MessageTypeToStatus()),
		Conclusion:      s.Result,
		CompletedAt:     s.FinishTime,
		StartedAt:       s.RunnerAssignTime,
		Name:            s.JobDisplayName,
		GithubRunnerID:  s.RunnerID,
		RunnerName:      s.RunnerName,
		RepositoryName:  s.RepositoryName,
		RepositoryOwner: s.OwnerName,
		Labels:          s.RequestLabels,
	}
}
