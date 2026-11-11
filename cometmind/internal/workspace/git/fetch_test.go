package git

import (
	"context"
	"testing"
)

func TestFetchRequiresRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Fetch(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for non-repo")
	}
}

func TestFetchAllPruneOnLocalRepo(t *testing.T) {
	dir := initRepo(t)
	result, err := Fetch(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("fetch result = %+v", result)
	}
}
