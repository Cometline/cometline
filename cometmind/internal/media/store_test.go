package media_test

import (
	"errors"
	"io"
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

	file, mt, err := media.Open("sess1", ref.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	opened, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || mt != "image/png" || string(opened) != string(png) {
		t.Fatalf("Open got mt=%q len=%d err=%v", mt, len(opened), err)
	}

	if err := media.DeleteSession("sess1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, err := media.Read("sess1", ref.ID); err == nil {
		t.Fatal("expected missing after delete")
	}
}

func TestRegisterVideoAndCopyFile(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())

	mp4 := []byte("ftypisom" + "xxxxxxxx")
	copy(mp4, []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p'})
	ref, err := media.RegisterBytesLimited("sess-video", "video/mp4", "clip", mp4, media.MaxVideoBytes)
	if err != nil {
		t.Fatalf("RegisterBytesLimited: %v", err)
	}
	if ref.Kind != media.KindVideo {
		t.Fatalf("kind = %q", ref.Kind)
	}
	copied, err := media.CopyFile("sess-copy", ref.Path, ref.MediaType, ref.Alt)
	if err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if copied.ID == ref.ID {
		t.Fatal("copied media reused the same id")
	}
	mt, data, err := media.Read("sess-copy", copied.ID)
	if err != nil {
		t.Fatalf("Read copied: %v", err)
	}
	if mt != "video/mp4" || string(data) != string(mp4) {
		t.Fatalf("copied media mt=%q len=%d", mt, len(data))
	}
	if err := media.DeleteFile("sess-video", ref.ID); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, _, err := media.Read("sess-video", ref.ID); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("Read after delete = %v", err)
	}
}
