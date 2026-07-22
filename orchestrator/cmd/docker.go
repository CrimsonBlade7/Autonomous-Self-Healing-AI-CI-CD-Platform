package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log/slog"
	_ "log/slog"
	"os"
	"path/filepath"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Builds an image from src
func buildImage(ctx context.Context, src string) error {

	pr, pw := io.Pipe()
	defer pr.Close()
	go func() {
		tw := tar.NewWriter(pw)
		defer tw.Close()

		err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}

			// Skip root
			if relPath == "." {
				return nil
			}

			fi, err := d.Info()
			if err != nil {
				return fmt.Errorf("failed to get info for %s: %w", path, err)
			}

			header, err := tar.FileInfoHeader(fi, d.Name())
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relPath)

			err = tw.WriteHeader(header)
			if err != nil {
				return err
			}

			if d.Type().IsRegular() {
				file, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer file.Close()
				_, err = io.Copy(tw, file)
				if err != nil {
					return err
				}
			}

			return nil
		})

		_ = pw.CloseWithError(err)
	}()

	imageResult, err := mobyClient.ImageBuild(ctx, pr, client.ImageBuildOptions{
		Tags:       []string{src},
		Dockerfile: "Dockerfile",
	})
	if err != nil {
		return err
	}
	defer imageResult.Body.Close()

	_, err = io.Copy(os.Stdout, imageResult.Body)
	if err != nil {
		return err
	}

	return nil
}

func buildContainer(ctx context.Context, src string) error {

	cont, err := mobyClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Cmd: []string{ /*initial command*/ },
		},
		Name:  src,
		Image: src,
	})
	if err != nil {
		return err
	}

	defer func() {
		_, err = mobyClient.ContainerRemove(ctx, cont.ID, client.ContainerRemoveOptions{
			RemoveVolumes: true,
		})
		if err != nil {
			panic(err)
		}
	}()

	_, err = mobyClient.ContainerStart(ctx, cont.ID, client.ContainerStartOptions{})
	if err != nil {
		return err
	}

	response := mobyClient.ContainerWait(ctx, cont.ID, client.ContainerWaitOptions{})
	select {
	case <-response.Result:
		slog.Info("Container completed")
	case err = <- response.Error:
		slog.Error("Container completed", "error", err)
	}

	logs, err := mobyClient.ContainerLogs(ctx, cont.ID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return err
	}
	defer logs.Close()

	bytes, err := io.ReadAll(logs)
	if err != nil {
		return err
	}

	slog.Info("Container Logs", slog.String("logs", string(bytes)))

	return nil
}
