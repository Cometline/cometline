package media_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/media"
)

func TestRegisterReadDelete(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())

	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}
	ref, err := media.RegisterBytes("sess1", "image/png", "shot", png)
	if err != nil {
		t.Fatalf("RegisterBytes: %v", err)
	}
	if ref.ID == "" || ref.Path == "" {
		t.Fatalf("ref incomplete: %+v", ref)
	}
	mt, data, err := media.Read("sess1", ref.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if mt != "image/png" || string(data) != string(png) {
		t.Fatalf("Read got mt=%q len=%d", mt, len(data))
	}

	src := filepath.Join(t.TempDir(), "shot.jpg")
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}
	if err := os.WriteFile(src, jpeg, 0o600); err != nil {
		t.Fatal(err)
	}
	ref2, err := media.RegisterFile("sess1", src, "from file")
	if err != nil {
		t.Fatalf("RegisterFile: %v", err)
	}
	if ref2.MediaType != "image/jpeg" {
		t.Fatalf("media type = %q", ref2.MediaType)
	}

	if err := media.DeleteSession("sess1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, err := media.Read("sess1", ref.ID); err == nil {
		t.Fatal("expected missing after delete")
	}
}
