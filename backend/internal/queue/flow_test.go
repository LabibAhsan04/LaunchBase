package queue_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/queue"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

func TestQueuedDeploymentBecomesReady(t *testing.T) {
	db := testDB(t)
	stream := testStream(t)
	_, depID := seed(t, db, "Demo")
	pending, err := db.HasPendingOutbox(context.Background(), depID)
	if err != nil || !pending {
		t.Fatalf("outbox pending=%v err=%v", pending, err)
	}
	calls := &recorder{}
	proc := processor(db, readyRun(calls), calls)
	cancel := startWorker(t, db, stream, proc, time.Hour)
	defer cancel()

	dep := waitStatus(t, db, depID, "READY")
	if dep.URL != "http://127.0.0.1:9/" || dep.ContainerName != deployments.ContainerName(depID) {
		t.Fatalf("%+v", dep)
	}
	if calls.count() != 1 {
		t.Fatalf("runs %d", calls.count())
	}

	// A second copy of the same job must not start another container.
	res, err := proc.Execute(context.Background(), sampleJob(t, db, depID))
	if err != nil || res.Dead || res.Busy {
		t.Fatalf("duplicate %+v %v", res, err)
	}
	if calls.count() != 1 {
		t.Fatalf("duplicate ran, runs %d", calls.count())
	}
}

func TestTransientRetryThenReady(t *testing.T) {
	db := testDB(t)
	stream := testStream(t)
	_, depID := seed(t, db, "Demo")
	calls := &recorder{}
	proc := processor(db, func(_ context.Context, opt deployments.Options) (deployments.Result, error) {
		n := calls.add()
		if n == 1 {
			if err := opt.OnStatus(deployments.StatusCloning); err != nil {
				return deployments.Result{}, err
			}
			return deployments.Result{}, deployments.Transient(fmt.Errorf("connection refused"))
		}
		return walkReady(opt)
	}, calls)
	cancel := startWorker(t, db, stream, proc, time.Hour)
	defer cancel()
	waitStatus(t, db, depID, "READY")
	if calls.count() != 2 {
		t.Fatalf("runs %d", calls.count())
	}
}

func TestBuildFailureIsNotRetried(t *testing.T) {
	db := testDB(t)
	stream := testStream(t)
	_, depID := seed(t, db, "Demo")
	calls := &recorder{}
	proc := processor(db, func(_ context.Context, opt deployments.Options) (deployments.Result, error) {
		calls.add()
		if err := opt.OnStatus(deployments.StatusCloning); err != nil {
			return deployments.Result{}, err
		}
		return deployments.Result{}, fmt.Errorf("docker build: missing dockerfile")
	}, calls)
	cancel := startWorker(t, db, stream, proc, time.Hour)
	defer cancel()
	dep := waitStatus(t, db, depID, "FAILED")
	if dep.URL != "" || calls.count() != 1 {
		t.Fatalf("dep=%+v runs=%d", dep, calls.count())
	}
	waitDead(t, stream, 1)
}

func TestTransientGivesUp(t *testing.T) {
	db := testDB(t)
	stream := testStream(t)
	_, depID := seed(t, db, "Demo")
	calls := &recorder{}
	proc := processor(db, func(_ context.Context, opt deployments.Options) (deployments.Result, error) {
		calls.add()
		return deployments.Result{}, deployments.Transient(fmt.Errorf("connection reset by peer"))
	}, calls)
	proc.MaxAttempts = 2
	cancel := startWorker(t, db, stream, proc, time.Hour)
	defer cancel()
	waitStatus(t, db, depID, "FAILED")
	if calls.count() != 2 {
		t.Fatalf("runs %d", calls.count())
	}
	waitDead(t, stream, 1)
}

func TestRestartReclaimsUnackedJob(t *testing.T) {
	db := testDB(t)
	stream := testStream(t)
	_, depID := seed(t, db, "Demo")
	if err := stream.EnsureGroup(context.Background()); err != nil {
		t.Fatal(err)
	}
	ok, err := db.PublishPending(context.Background(), func(payload []byte) error {
		return stream.Publish(context.Background(), payload)
	})
	if err != nil || !ok {
		t.Fatalf("publish ok=%v err=%v", ok, err)
	}
	if _, err := stream.Read(context.Background(), "crashed", time.Second); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)

	calls := &recorder{}
	proc := processor(db, readyRun(calls), calls)
	cancel := startWorker(t, db, stream, proc, 20*time.Millisecond)
	defer cancel()
	waitStatus(t, db, depID, "READY")
	if calls.count() != 1 {
		t.Fatalf("runs %d", calls.count())
	}
}

func TestStoppingTheWorkerLeavesTheNextJobQueued(t *testing.T) {
	db := testDB(t)
	stream := testStream(t)
	_, first := seed(t, db, "Demo")
	calls := &recorder{}
	proc := processor(db, readyRun(calls), calls)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = (&queue.Consumer{
			Store: db, Stream: stream, Handle: proc.Execute, Name: "stop-test",
			Idle: time.Hour, Block: 30 * time.Millisecond,
		}).Run(ctx)
		close(done)
	}()
	waitStatus(t, db, first, "READY")
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}

	_, second := seed(t, db, "Other")
	time.Sleep(150 * time.Millisecond)
	dep, err := db.GetDeployment(context.Background(), store.LocalUserID, second)
	if err != nil || dep.Status != "QUEUED" {
		t.Fatalf("after stop %+v %v", dep, err)
	}
	cancel2 := startWorker(t, db, stream, proc, time.Hour)
	defer cancel2()
	waitStatus(t, db, second, "READY")
}

func TestInProgressDeploymentIsRebuiltOnce(t *testing.T) {
	db := testDB(t)
	_, depID := seed(t, db, "Demo")
	cur := "QUEUED"
	for _, next := range []string{"CLONING", "BUILDING"} {
		if err := db.Transition(context.Background(), depID, cur, next); err != nil {
			t.Fatal(err)
		}
		cur = next
	}
	calls := &recorder{}
	proc := processor(db, readyRun(calls), calls)
	res, err := proc.Execute(context.Background(), sampleJob(t, db, depID))
	if err != nil || res.Dead || res.Busy {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	dep, err := db.GetDeployment(context.Background(), store.LocalUserID, depID)
	if err != nil || dep.Status != "READY" {
		t.Fatalf("%+v %v", dep, err)
	}
	if calls.count() != 1 || len(calls.removed()) != 1 {
		t.Fatalf("runs=%d removed=%v", calls.count(), calls.removed())
	}
}

func TestStatusJumpFailsTheDeployment(t *testing.T) {
	db := testDB(t)
	_, depID := seed(t, db, "Demo")
	calls := &recorder{}
	proc := processor(db, func(_ context.Context, opt deployments.Options) (deployments.Result, error) {
		calls.add()
		if err := opt.OnStatus(deployments.StatusReady); err != nil {
			return deployments.Result{}, err
		}
		return deployments.Result{URL: "http://127.0.0.1:1/"}, nil
	}, calls)
	res, err := proc.Execute(context.Background(), sampleJob(t, db, depID))
	if err != nil || !res.Dead {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	dep, err := db.GetDeployment(context.Background(), store.LocalUserID, depID)
	if err != nil || dep.Status != "FAILED" || dep.URL != "" {
		t.Fatalf("%+v %v", dep, err)
	}
}

func TestLockStopsASecondRun(t *testing.T) {
	db := testDB(t)
	_, depID := seed(t, db, "Demo")
	ok, err := db.TryLock(context.Background(), depID, "other-worker", time.Hour)
	if err != nil || !ok {
		t.Fatalf("lock ok=%v err=%v", ok, err)
	}
	calls := &recorder{}
	proc := processor(db, readyRun(calls), calls)
	res, err := proc.Execute(context.Background(), sampleJob(t, db, depID))
	if err != nil || !res.Busy || calls.count() != 0 {
		t.Fatalf("res=%+v err=%v runs=%d", res, err, calls.count())
	}
	if err := db.Unlock(context.Background(), depID, "other-worker"); err != nil {
		t.Fatal(err)
	}
	res, err = proc.Execute(context.Background(), sampleJob(t, db, depID))
	if err != nil || res.Busy || res.Dead {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if calls.count() != 1 {
		t.Fatalf("runs %d", calls.count())
	}
}

func TestWorkerHealth(t *testing.T) {
	gate := &queue.Gate{}
	h := queue.Handler(gate, func(context.Context) error { return nil })
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Fatalf("health %d %s", rr.Code, rr.Body.String())
	}
	gate.Drain()
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready %d %s", rr.Code, rr.Body.String())
	}
}

func processor(db *store.DB, run func(context.Context, deployments.Options) (deployments.Result, error), rem deployments.Remover) *deployments.Processor {
	return &deployments.Processor{
		Store:       db,
		Run:         run,
		Remove:      rem,
		Owner:       "test-worker",
		MaxAttempts: 3,
		Stale:       time.Hour,
		Backoff:     func(int) time.Duration { return -time.Second },
	}
}

func readyRun(c *recorder) func(context.Context, deployments.Options) (deployments.Result, error) {
	return func(_ context.Context, opt deployments.Options) (deployments.Result, error) {
		c.add()
		return walkReady(opt)
	}
}

func walkReady(opt deployments.Options) (deployments.Result, error) {
	for _, st := range []deployments.Status{
		deployments.StatusCloning,
		deployments.StatusDetecting,
		deployments.StatusBuilding,
		deployments.StatusStarting,
		deployments.StatusHealthChecking,
		deployments.StatusReady,
	} {
		if err := opt.OnStatus(st); err != nil {
			return deployments.Result{}, err
		}
	}
	return deployments.Result{
		Image:     deployments.ImageName(opt.ID),
		Container: deployments.ContainerName(opt.ID),
		URL:       "http://127.0.0.1:9/",
	}, nil
}

func startWorker(t *testing.T, db *store.DB, stream *queue.Stream, proc *deployments.Processor, idle time.Duration) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = (&queue.Consumer{
			Store: db, Stream: stream, Handle: proc.Execute, Name: proc.Owner,
			Idle: idle, Block: 30 * time.Millisecond,
		}).Run(ctx)
	}()
	return cancel
}

func seed(t *testing.T, db *store.DB, name string) (string, string) {
	t.Helper()
	svc := &service.Service{Store: db}
	project, err := svc.CreateProject(context.Background(), name, "https://github.com/acme/web", "main")
	if err != nil {
		t.Fatal(err)
	}
	dep, err := svc.CreateDeployment(context.Background(), project.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if dep.Status != "QUEUED" {
		t.Fatalf("status %s", dep.Status)
	}
	return project.ID, dep.ID
}

func sampleJob(t *testing.T, db *store.DB, depID string) queue.Job {
	t.Helper()
	dep, err := db.GetDeployment(context.Background(), store.LocalUserID, depID)
	if err != nil {
		t.Fatal(err)
	}
	return queue.Job{
		Version: queue.JobVersion, DeploymentID: dep.ID, ProjectID: dep.ProjectID,
		RepoURL: dep.RepoURL, Branch: dep.Branch, Attempt: 1,
	}
}

func waitStatus(t *testing.T, db *store.DB, id, want string) store.Deployment {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	var last store.Deployment
	for time.Now().Before(deadline) {
		dep, err := db.GetDeployment(context.Background(), store.LocalUserID, id)
		if err == nil {
			last = dep
			if dep.Status == want {
				return dep
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("status %s, want %s (error %s)", last.Status, want, last.ErrorSummary)
	return last
}

func waitDead(t *testing.T, stream *queue.Stream, min int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var n int64
	for time.Now().Before(deadline) {
		var err error
		n, err = stream.DeadCount(context.Background())
		if err == nil && n >= min {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("dead letters %d, want at least %d", n, min)
}

type recorder struct {
	mu    sync.Mutex
	n     int
	names []string
}

func (r *recorder) add() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.n++
	return r.n
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}

func (r *recorder) RemoveContainer(_ context.Context, name string) error {
	r.mu.Lock()
	r.names = append(r.names, name)
	r.mu.Unlock()
	return nil
}

func (r *recorder) removed() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.names...)
	return out
}

func testStream(t *testing.T) *queue.Stream {
	t.Helper()
	rawURL := os.Getenv("REDIS_URL")
	if rawURL == "" {
		rawURL = "redis://localhost:6379/0"
	}
	var raw [4]byte
	_, _ = rand.Read(raw[:])
	stream, err := queue.OpenNamed(rawURL, "launchbase:test:"+hex.EncodeToString(raw[:]), "workers")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := stream.Ping(ctx); err != nil {
		t.Fatalf("redis is not running (%v). start it with make dev-infra", err)
	}
	t.Cleanup(func() {
		stream.Delete(context.Background())
		_ = stream.Close()
	})
	return stream
}

func testDB(t *testing.T) *store.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://launchbase:launchbase@localhost:5433/launchbase?sslmode=disable"
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		t.Fatalf("postgres is not running (%v). start it with make dev-infra", err)
	}
	var raw [4]byte
	_, _ = rand.Read(raw[:])
	schema := "lb_" + hex.EncodeToString(raw[:])
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+store.QuoteIdent(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + store.QuoteIdent(schema) + " CASCADE")
		_ = admin.Close()
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := store.Open(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}
