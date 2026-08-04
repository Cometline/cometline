package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/paths"
)

const backupNamePrefix = "cometmind-backup-"

// Config controls one archive run.
type Config struct {
	DestinationDir string
	MaxBackups     int
}

// Result summarizes a completed archive.
type Result struct {
	Path        string
	FilesZipped int
	RemovedOld  int
}

// Archiver creates zip backups of the CometMind data directory.
type Archiver struct {
	DB *sql.DB
}

// Run zips ~/.cometmind into destinationDir and rotates old archives.
func (a *Archiver) Run(ctx context.Context, cfg Config) (Result, error) {
	dest := strings.TrimSpace(cfg.DestinationDir)
	if dest == "" {
		return Result{}, fmt.Errorf("backup destination directory is required")
	}
	dest, err := filepath.Abs(dest)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return Result{}, fmt.Errorf("create backup destination: %w", err)
	}
	dest, err = filepath.EvalSymlinks(dest)
	if err != nil {
		return Result{}, fmt.Errorf("resolve backup destination: %w", err)
	}

	dataDir, err := paths.DataDir()
	if err != nil {
		return Result{}, err
	}
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return Result{}, err
	}
	dataDir, err = filepath.EvalSymlinks(dataDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolve CometMind data directory: %w", err)
	}
	if pathWithin(dest, dataDir) {
		return Result{}, fmt.Errorf("backup destination must be outside the CometMind data directory")
	}

	tmpDir, err := os.MkdirTemp("", "cometmind-backup-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmpDir)

	dbSnapshot := ""
	if a != nil && a.DB != nil {
		dbSnapshot, err = snapshotDatabase(ctx, a.DB, tmpDir)
		if err != nil {
			return Result{}, fmt.Errorf("database snapshot: %w", err)
		}
	}

	stamp := time.Now().Format("2006-01-02T150405")
	outFile, outPath, err := createBackupFile(dest, stamp)
	if err != nil {
		return Result{}, err
	}
	if err := writeArchive(dataDir, dbSnapshot, outFile); err != nil {
		_ = outFile.Close()
		_ = os.Remove(outPath)
		return Result{}, err
	}
	if err := outFile.Close(); err != nil {
		_ = os.Remove(outPath)
		return Result{}, err
	}
	if err := os.Chmod(outPath, 0o600); err != nil {
		return Result{}, err
	}

	removed, err := rotate(dest, cfg.MaxBackups)
	if err != nil {
		return Result{}, err
	}

	files, err := countZipEntries(outPath)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Path:        outPath,
		FilesZipped: files,
		RemovedOld:  removed,
	}, nil
}

func snapshotDatabase(ctx context.Context, db *sql.DB, tmpDir string) (string, error) {
	snapshotPath := filepath.Join(tmpDir, "cometmind.db")
	escaped := strings.ReplaceAll(snapshotPath, "'", "''")
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		return "", err
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("ATTACH DATABASE '%s' AS backup_snapshot", escaped)); err != nil {
		return "", err
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "DETACH DATABASE backup_snapshot")
	if _, err := conn.ExecContext(ctx, "DELETE FROM backup_snapshot.assistant_provider_states"); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return "", err
		}
	}
	return snapshotPath, nil
}

func writeArchive(dataDir, dbSnapshot string, out io.Writer) error {
	zw := zip.NewWriter(out)
	err := filepath.WalkDir(dataDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		// WalkDir uses lstat and can discover a symlink even when its target no
		// longer exists. The archiver follows symlinks when adding files, so skip
		// only dangling links instead of failing the entire backup.
		if d.Type()&os.ModeSymlink != 0 {
			if _, err := os.Stat(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			}
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		src := path
		if rel == "cometmind.db" && dbSnapshot != "" {
			src = dbSnapshot
		}
		if err := addFileToZip(zw, src, rel); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func createBackupFile(dest, stamp string) (*os.File, string, error) {
	for i := 1; i < 1000; i++ {
		name := backupNamePrefix + stamp + ".zip"
		if i > 1 {
			name = backupNamePrefix + stamp + fmt.Sprintf("-%03d", i-1) + ".zip"
		}
		outPath := filepath.Join(dest, name)
		f, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return f, outPath, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("allocate unique backup path")
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func addFileToZip(zw *zip.Writer, srcPath, entryName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = entryName
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

func rotate(dest string, maxBackups int) (int, error) {
	if maxBackups <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return 0, err
	}
	var backups []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.HasPrefix(name, backupNamePrefix) && strings.HasSuffix(name, ".zip") {
			backups = append(backups, filepath.Join(dest, name))
		}
	}
	sort.Strings(backups)
	if len(backups) <= maxBackups {
		return 0, nil
	}
	toRemove := len(backups) - maxBackups
	removed := 0
	for i := 0; i < toRemove; i++ {
		if err := os.Remove(backups[i]); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func countZipEntries(path string) (int, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	return len(r.File), nil
}
