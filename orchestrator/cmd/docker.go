package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// Cleans up old images
func cleanOldImages(ctx context.Context, cli *client.Client) error {
	images, err := cli.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		return err
	}
	for _, item := range images.Items {
		_, err = cli.ImageRemove(ctx, item.ID, client.ImageRemoveOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// Builds an image from src
func buildImage(ctx context.Context, cli *client.Client, tag, srcPath string) error {

	pr, pw := io.Pipe()
	defer pr.Close()
	go func() {
		tw := tar.NewWriter(pw)
		defer tw.Close()

		err := filepath.WalkDir(srcPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(srcPath, path)
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
					return err
				}
				defer file.Close()
				_, err = io.Copy(tw, file)
				if err != nil {
					return err
				}
			}

			return nil
		})

		err = pw.CloseWithError(err)
		if err != nil {
			slog.Error("Pipe writter failed to close", "error", err)
			return
		}
	}()

	// TODO: Generate dockerfile

	imageResult, err := cli.ImageBuild(ctx, pr, client.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
	})
	if err != nil {
		return err
	}
	defer imageResult.Body.Close()

	// TODO: temp for testing
	// imageResult.Body needs to be drained when complete
	_, err = io.Copy(os.Stdout, imageResult.Body)
	if err != nil {
		return err
	}

	return nil
}

func runContainer(ctx context.Context, cli *client.Client, tag string) error {

	cont, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{},
		Name:   tag,
		Image:  tag,
	})
	if err != nil {
		return err
	}

	defer func() {
		_, err = cli.ContainerRemove(ctx, cont.ID, client.ContainerRemoveOptions{
			RemoveVolumes: true,
		})
		if err != nil {
			slog.Error("Failed to remove container", "error", err)
			return
		}
	}()

	_, err = cli.ContainerStart(ctx, cont.ID, client.ContainerStartOptions{})
	if err != nil {
		return err
	}

	response := cli.ContainerWait(ctx, cont.ID, client.ContainerWaitOptions{})
	select {
	case <-response.Result:
		slog.Info("Container completed")
	case err = <-response.Error:
		slog.Error("Container completed", "error", err)
	}

	logs, err := cli.ContainerLogs(ctx, cont.ID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
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
