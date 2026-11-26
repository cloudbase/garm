// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.
package scaleset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/stretchr/testify/mock"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common/mocks"
	"github.com/cloudbase/garm/util/github/scalesets"
)

const jobTestRegistrationToken = "registration-token"

type jobStoreFixture struct {
	dbCommon.Store
	dbCommon.AdmissionStore
	policy   params.AdmissionConfig
	set      params.ScaleSet
	jobs     []params.AdmissionJob
	drained  bool
	desired  int
	records  int
	acquires int
}

func (s *jobStoreFixture) GetScaleSetByID(context.Context, uint) (params.ScaleSet, error) {
	return s.set, nil
}

func (s *jobStoreFixture) AdmissionForProvider(context.Context, string) (params.AdmissionConfig, bool, error) {
	return s.policy, true, nil
}

func (s *jobStoreFixture) GetAdmissionCredit(context.Context, string, string) (params.AdmissionReservation, []params.AdmissionJob, error) {
	return params.AdmissionReservation{}, append([]params.AdmissionJob(nil), s.jobs...), nil
}

func (s *jobStoreFixture) RecordAdmissionAssignment(_ context.Context, domain, owner, request, id string) (params.AdmissionJob, error) {
	s.records++
	for _, j := range s.jobs {
		if j.RequestID == request && j.JobID == id {
			return j, nil
		}
	}
	j := params.AdmissionJob{Domain: domain, Owner: owner, RequestID: request, JobID: id, State: "offered"}
	s.jobs = append(s.jobs, j)
	return j, nil
}

func (s *jobStoreFixture) MarkAdmissionAcquired(_ context.Context, _, _, request, id string) error {
	for i := range s.jobs {
		if s.jobs[i].RequestID == request && s.jobs[i].JobID == id {
			s.jobs[i].State = jobStateAcquired
			return nil
		}
	}
	return dbCommon.ErrAdmissionNotFound
}

func (s *jobStoreFixture) CompleteAdmissionJob(_ context.Context, _, _, request, id string) error {
	for i := range s.jobs {
		if s.jobs[i].RequestID == request && s.jobs[i].JobID == id {
			s.jobs[i].Terminal = true
			return nil
		}
	}
	return dbCommon.ErrAdmissionNotFound
}

func (s *jobStoreFixture) BeginAdmissionAcquire(context.Context, string, string, string, string) (bool, error) {
	s.acquires++
	return true, nil
}

func (s *jobStoreFixture) SetAdmissionDrain(_ context.Context, _ string, b bool) error {
	s.drained = b
	return nil
}

func (s *jobStoreFixture) SetScaleSetDesiredRunnerCount(_ context.Context, _ uint, n int) error {
	s.desired = n
	return nil
}

func (s *jobStoreFixture) CreateOrUpdateScaleSetJob(_ context.Context, j params.ScaleSetJob) (params.ScaleSetJob, error) {
	return j, nil
}

func jobMessage(t *testing.T, jobs []params.ScaleSetJobMessage, stats params.RunnerScaleSetStatistic) params.RunnerScaleSetMessage {
	t.Helper()
	data, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	return params.RunnerScaleSetMessage{Body: string(data), Statistics: &stats}
}

func TestJobAdmissionMixedCompletionAndNextAssignment(t *testing.T) {
	store := &jobStoreFixture{policy: params.AdmissionConfig{Domain: "domain", Protocol: jobAdmissionProtocol, Ready: true, HealthyUntil: time.Now().Add(time.Minute)}, set: params.ScaleSet{ID: 1, Enabled: true, ProviderName: "p"}, jobs: []params.AdmissionJob{{RequestID: "1", JobID: "a", State: jobStateAcquired, InstanceName: "old"}}}
	worker := &Worker{ctx: context.Background(), store: store, scaleSet: store.set, jobScaleSetID: 1}
	msg := jobMessage(t, []params.ScaleSetJobMessage{{MessageType: params.MessageTypeJobCompleted, RunnerRequestID: 1, JobID: "a"}, {MessageType: params.MessageTypeJobAssigned, RunnerRequestID: 2, JobID: "b"}}, params.RunnerScaleSetStatistic{TotalAssignedJobs: 1})
	if err := worker.handleJobAdmissionMessage(msg, nil, ""); err != nil {
		t.Fatal(err)
	}
	if store.drained || len(store.jobs) != 2 || !store.jobs[0].Terminal || store.jobs[1].State != jobStateAcquired || store.desired != 1 {
		t.Fatalf("unexpected result: %+v", store)
	}
}

func TestAvailableOffersDoNotClaimBusyOrUnhealthyCredit(t *testing.T) {
	for _, busy := range []bool{false, true} {
		t.Run(map[bool]string{false: "stale", true: "busy"}[busy], func(t *testing.T) {
			store := &jobStoreFixture{policy: params.AdmissionConfig{Domain: "domain", Protocol: jobAdmissionProtocol}, set: params.ScaleSet{ID: 1, Enabled: true, ProviderName: "p"}}
			if busy {
				store.policy.Ready = true
				store.policy.HealthyUntil = time.Now().Add(time.Minute)
				store.jobs = []params.AdmissionJob{{RequestID: "1", JobID: "a", State: jobStateAcquired}}
			}
			worker := &Worker{ctx: context.Background(), store: store, scaleSet: store.set, jobScaleSetID: 1}
			msg := jobMessage(t, []params.ScaleSetJobMessage{{MessageType: params.MessageTypeJobAvailable, RunnerRequestID: 2, JobID: "b"}, {MessageType: params.MessageTypeJobAvailable, RunnerRequestID: 3, JobID: "c"}}, params.RunnerScaleSetStatistic{})
			if err := worker.handleJobAdmissionMessage(msg, nil, ""); err != nil {
				t.Fatal(err)
			}
			if store.records != 0 || store.acquires != 0 || store.drained {
				t.Fatalf("unselected offers changed admission: %+v", store)
			}
		})
	}
}

type jobListenerFixture struct {
	*listenerTestHelper
	prepared atomic.Bool
	polls    atomic.Int64
	finished atomic.Int64
}

func (h *jobListenerFixture) prepareJobSession() (bool, error) {
	h.prepared.Store(true)
	return true, nil
}

func (*jobListenerFixture) validateJobSession(params.RunnerScaleSetStatistic, bool) error { return nil }

func (h *jobListenerFixture) authorizeJobPoll() (uint, int64, error) {
	if h.polls.Add(1) == 1 {
		return 1, 1, nil
	}
	return 0, 0, nil
}
func (h *jobListenerFixture) finishJobPoll(int64) error { h.finished.Add(1); return nil }
func (*jobListenerFixture) handleJobAdmissionMessage(params.RunnerScaleSetMessage, *scalesets.ScaleSetClient, string) error {
	return nil
}

func TestJobListenerEscrowsBeforeSessionAndSendsZeroAndOne(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	headers := make(chan string, 2)
	var base string
	fixture := &jobListenerFixture{listenerTestHelper: &listenerTestHelper{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/runner-registration"):
			fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxMDI0NDQ4MDB9."}`, base)
		case strings.HasSuffix(r.URL.Path, "/sessions") && r.Method == http.MethodPost:
			if !fixture.prepared.Load() {
				t.Error("session created before escrow")
			}
			fmt.Fprintf(w, `{"sessionId":"00000000-0000-0000-0000-000000000001","messageQueueUrl":%q,"messageQueueAccessToken":"test-token","statistics":{}}`, base+"/queue")
		case r.URL.Path == "/queue":
			select {
			case headers <- r.Header.Get("X-ScaleSetMaxCapacity"):
			default:
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	base = srv.URL
	baseURL, err := url.Parse(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	gh := mocks.NewGithubClient(t)
	gh.On("GetEntity").Return(params.ForgeEntity{}).Maybe()
	gh.On("GithubBaseURL").Return(baseURL).Maybe()
	token := jobTestRegistrationToken
	gh.On("CreateEntityRegistrationToken", mock.Anything).Return(&github.RegistrationToken{Token: &token, ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)}}, (*github.Response)(nil), nil).Maybe()
	client, err := scalesets.NewClient(gh)
	if err != nil {
		t.Fatal(err)
	}
	fixture.client = client
	listener := newListener(ctx, fixture)
	if err := listener.Start(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1", "0"} {
		select {
		case got := <-headers:
			if got != want {
				t.Fatalf("capacity=%q want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("poll not received")
		}
	}
	cancel()
	select {
	case <-listener.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not stop")
	}
}

func TestDisabledCreditedSetContinuesZeroCapacityPolling(t *testing.T) {
	store := &jobStoreFixture{policy: params.AdmissionConfig{Domain: "domain", Protocol: jobAdmissionProtocol}, set: params.ScaleSet{ID: 1, Enabled: false, ProviderName: "p"}}
	worker := &Worker{ctx: context.Background(), store: store, scaleSet: store.set, jobScaleSetID: 1}
	if !worker.listenerEligibleLocked() {
		t.Fatal("existing credit must keep lifecycle listener eligible")
	}
	capacity, epoch, err := worker.authorizeJobPoll()
	if err != nil || capacity != 0 || epoch != 0 {
		t.Fatalf("disabled poll: capacity=%d epoch=%d err=%v", capacity, epoch, err)
	}
}

func TestConcurrentReadinessAndScaleSetUpdates(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	image := strings.Repeat("a", 64)
	policy := params.AdmissionConfig{Domain: "d", Provider: "p", Generation: 1, Project: "project", Image: image, Flavor: "profile", OSArch: "amd64", InstanceType: "virtual-machine", Class: params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1, Jobs: 1}}
	store := &readinessStore{}
	provider := &readinessProvider{now: &now}
	set := params.ScaleSet{ID: 1, Image: image, Flavor: "profile", OSArch: "amd64"}
	worker := &Worker{ctx: context.Background(), store: store, provider: provider, scaleSet: set, readinessNow: func() time.Time { return now }}
	worker.scaleSetView.Store(&set)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			worker.mux.Lock()
			worker.scaleSet = set
			worker.scaleSetView.Store(&set)
			worker.mux.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			worker.mux.Lock()
			_ = worker.checkAdmissionReadiness(store, policy)
			worker.mux.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = worker.checkAdmissionReadinessFor(store, policy, worker.GetScaleSet())
		}
	}()
	wg.Wait()
}

func TestMalformedPositivePollRemainsPendingAfterRestart(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"empty object", 200, `{}`}, {"null", 200, `null`}, {"unexpected success", 201, `{}`}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			headers := make(chan string, 2)
			var base string
			fixture := &jobListenerFixture{listenerTestHelper: &listenerTestHelper{}}
			var requests atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/actions/runner-registration"):
					fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxMDI0NDQ4MDB9."}`, base)
				case strings.HasSuffix(r.URL.Path, "/sessions") && r.Method == http.MethodPost:
					fmt.Fprintf(w, `{"sessionId":"00000000-0000-0000-0000-000000000001","messageQueueUrl":%q,"messageQueueAccessToken":"test-token","statistics":{}}`, base+"/queue")
				case r.URL.Path == "/queue":
					select {
					case headers <- r.Header.Get("X-ScaleSetMaxCapacity"):
					default:
					}
					if requests.Add(1) == 1 {
						w.WriteHeader(tc.status)
						_, _ = w.Write([]byte(tc.body))
						return
					}
					w.WriteHeader(http.StatusAccepted)
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer srv.Close()
			base = srv.URL
			baseURL, err := url.Parse(base + "/")
			if err != nil {
				t.Fatal(err)
			}
			gh := mocks.NewGithubClient(t)
			gh.On("GetEntity").Return(params.ForgeEntity{}).Maybe()
			gh.On("GithubBaseURL").Return(baseURL).Maybe()
			token := jobTestRegistrationToken
			gh.On("CreateEntityRegistrationToken", mock.Anything).Return(&github.RegistrationToken{Token: &token, ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)}}, (*github.Response)(nil), nil).Maybe()
			client, err := scalesets.NewClient(gh)
			if err != nil {
				t.Fatal(err)
			}
			fixture.client = client
			listener := newListener(ctx, fixture)
			if err := listener.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-listener.Wait():
			case <-time.After(5 * time.Second):
				t.Fatal("invalid poll did not stop listener")
			}
			if fixture.finished.Load() != 0 {
				t.Fatal("uncertain positive poll was finished")
			}
			if err := listener.Start(); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"1", "0"} {
				select {
				case got := <-headers:
					if got != want {
						t.Fatalf("capacity=%q want %q", got, want)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("poll not received")
				}
			}
			cancel()
			select {
			case <-listener.Wait():
			case <-time.After(5 * time.Second):
				t.Fatal("listener did not stop")
			}
		})
	}
}
