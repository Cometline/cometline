package backup

import (
	"archive/zip"
	"context"
	"database/sql"
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
	Path       string
	FilesZipped int
	RemovedOld int
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

	dataDir, err := paths.DataDir()
	if err != nil {
		return Result{}, err
	}
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return Result{}, err
	}
	if strings.HasPrefix(dest, dataDir+string(filepath.Separator)) || dest == dataDir {
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
	outPath := uniqueBackupPath(dest, stamp)
	if err := writeArchive(dataDir, dbSnapshot, outPath); err != nil {
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
	query := fmt.Sprintf("VACUUM INTO '%s'", escaped)
	if _, err := db.ExecContext(ctx, query); err != nil {
		return "", err
	}
	return snapshotPath, nil
}

func writeArchive(dataDir, dbSnapshot, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	filesZipped := 0
	err = filepath.WalkDir(dataDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
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
		filesZipped++
		return nil
	})
	if err != nil {
		return err
	}
	_ = filesZipped
	return nil
}

func uniqueBackupPath(dest, stamp string) string {
	base := backupNamePrefix + stamp + ".zip"
	outPath := filepath.Join(dest, base)
	for i := 1; i < 1000; i++ {
		if _, err := os.Stat(outPath); os.IsNotExist(err) {
			return outPath
		}
		outPath = filepath.Join(dest, backupNamePrefix+stamp+fmt.Sprintf("-%03d", i)+".zip")
	}
	return outPath
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
