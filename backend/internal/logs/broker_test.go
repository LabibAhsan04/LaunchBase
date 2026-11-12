package logs_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

func TestBrokerOrdersAndRedacts(t *testing.T) {
	db := testDB(t)
	hub := logs.NewMemoryHub()
	broker := &logs.Broker{Sink: db, Hub: hub, Secrets: []string{"sekrit-value"}}
	svc := &service.Service{Store: db}
	project, err := svc.CreateProject(context.Background(), "Demo", "https://github.com/acme/web", "main")
	if err != nil {
		t.Fatal(err)
	}
	dep, err := svc.CreateDeployment(context.Background(), project.ID, "main")
	if err != nil {
		t.Fatal(err)
	}

	live, cancel, err := broker.Subscribe(context.Background(), dep.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	first, err := broker.Append(context.Background(), dep.ID, logs.SourceWorker, logs.LevelInfo, "hello")
	if err != nil {
		t.Fatal(err)
	}
	second, err := broker.Append(context.Background(), dep.ID, logs.SourceWorker, logs.LevelInfo, "token=sekrit-value")
	if err != nil {
		t.Fatal(err)
	}
	if first.Seq != 1 || second.Seq != 2 {
		t.Fatalf("seq %d %d", first.Seq, second.Seq)
	}
	if second.Message != "token=[redacted]" {
		t.Fatalf("message %q", second.Message)
	}

	got := []logs.Event{}
	deadline := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case e := <-live:
			got = append(got, e)
		case <-deadline:
			t.Fatal("timeout waiting for live events")
		}
	}
	if got[0].Seq != 1 || got[1].Seq != 2 {
		t.Fatalf("%+v", got)
	}

	history, err := broker.ListAfter(context.Background(), dep.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[1].Message != "token=[redacted]" {
		t.Fatalf("%+v", history)
	}

	// Reconnect after seq 1 should only return the second line.
	resume, err := broker.ListAfter(context.Background(), dep.ID, 1, 10)
	if err != nil || len(resume) != 1 || resume[0].Seq != 2 {
		t.Fatalf("resume %+v %v", resume, err)
	}
}

func TestUnauthorizedDeploymentHasNoLogs(t *testing.T) {
	db := testDB(t)
	broker := &logs.Broker{Sink: db, Hub: logs.NewMemoryHub()}
	svc := &service.Service{Store: db, Logs: broker}
	_, err := svc.ListLogs(context.Background(), "00000000-0000-4000-8000-000000000099", 0, 10)
	if err != store.ErrNotFound {
		t.Fatalf("err %v", err)
	}
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
