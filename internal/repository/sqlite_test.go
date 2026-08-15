package repository_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sandeepv/hoptrace/internal/probe"
	"github.com/sandeepv/hoptrace/internal/repository"
)

func TestSQLiteSaveGetList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	repo, err := repository.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	report := probe.ProbeReport{
		InitialURL: "https://example.com",
		TotalSteps: 1,
		Steps:      []probe.StepResult{{URL: "https://example.com"}},
		Summary: probe.Summary{
			TotalTimeMS: 42.5,
			FinalStatus: 200,
			FinalURL:    "https://example.com",
			FinalBytes:  12,
		},
	}

	ctx := context.Background()
	rec, err := repo.Save(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID == "" {
		t.Fatal("expected id")
	}

	got, err := repo.Get(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Report.InitialURL != report.InitialURL || got.Report.Summary.FinalStatus != 200 {
		t.Fatalf("get mismatch: %+v", got.Report)
	}

	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != rec.ID {
		t.Fatalf("list=%+v", list)
	}
}
