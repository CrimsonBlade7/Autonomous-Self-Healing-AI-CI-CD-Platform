package dockertools

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type RealTarBuilder struct {}

func (tb *RealTarBuilder) TarWorkspace(pw *io.PipeWriter, path string) (err error) {
	tw := tar.NewWriter(pw)
	defer func() {
		closeErr := tw.Close()
		if closeErr != nil {
			err = closeErr
		}
	}()

	err = filepath.WalkDir(path, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("Error while walking directory: %w", walkErr)
		}
		relPath, err := filepath.Rel(path, path)
		if err != nil {
			return fmt.Errorf("Failed create relative path %s/%s: %w", path, path, err)
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return fmt.Errorf("Failed to get info for %s: %w", path, err)
		}

		header, err := tar.FileInfoHeader(fi, d.Name())
		if err != nil {
			return fmt.Errorf("Failed to create file info header: %w", err)
		}
		header.Name = filepath.ToSlash(relPath)

		err = tw.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("Failed to write header: %w", err)
		}

		if d.Type().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("Failed to open file %s: %w", path, err)
			}
			defer func() {
				closeErr := file.Close()
				if closeErr != nil {
					err = closeErr
				}
			}()
			_, err = io.Copy(tw, file)
			if err != nil {
				return fmt.Errorf("Failed to write file contents to tar writer: %w", err)
			}
		}

		return err
	})
	return err
}