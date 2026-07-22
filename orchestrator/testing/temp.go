package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/moby/moby/client"
)

func main() {
	ctx := context.Background()

	// 1. Initialize modern Moby client
	cli, err := client.New(client.FromEnv)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer cli.Close()

	// 2. Target subfolder and target image tag
	subfolderPath, _ := filepath.Abs("./my-subfolder")
	imageTag := "my-app:latest"

	// 3. Pipe the tarball creation directly to the build context reader
	// io.Pipe avoids writing a temporary file to disk.
	pr, pw := io.Pipe()

	go func() {
		err := createTarContext(subfolderPath, pw)
		_ = pw.CloseWithError(err)
	}()

	// 4. Trigger image build
	buildOptions := client.ImageBuildOptions{
		Tags:       []string{imageTag},
		Dockerfile: "Dockerfile", // Path inside subfolder
		Remove:     true,
	}

	response, err := cli.ImageBuild(ctx, pr, buildOptions)
	if err != nil {
		log.Fatalf("Image build failed: %v", err)
	}
	defer response.Body.Close()

	// 5. Output build output stream
	_, err = io.Copy(os.Stdout, response.Body)
	if err != nil {
		log.Fatalf("Error reading response stream: %v", err)
	}

	log.Println("Build complete:", imageTag)
}

// createTarContext walks the target folder using WalkDir and writes a standard tar archive stream
func createTarContext(srcDir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path inside the subfolder
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip root directory entry itself
		if relPath == "." {
			return nil
		}

		// Retrieve FileInfo (only fetches stat when needed, e.g., for headers/symlinks)
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get info for %s: %w", path, err)
		}

		// Create standard tar header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// Format to forward slashes for POSIX tar header compatibility across OSes
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write header for %s: %w", header.Name, err)
		}

		// Write file contents if regular file
		if d.Type().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})
}

func makingAContainer() {
	newCont, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Image: fmt.Sprintf("%s:latest", name),
		Config: &container.Config{
			Cmd: []string{"echo", fmt.Sprintf("%s starting", name)},
		},
		Name: name,
	})
	if err != nil {
		panic(err)
	}

	defer func() {
		_, err = apiClient.ContainerRemove(ctx, newCont.ID, client.ContainerRemoveOptions{
			RemoveVolumes: true,
			RemoveLinks:   false,
			Force:         false,
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s container closed\n", name)
	}()

	var alpineID string
	for _, cont := range contsItems {
		if getContainerName(ctx, apiClient, cont.ID) == "alpine" {
			alpineID = cont.ID
		}
	}

	_, err = apiClient.ContainerStart(ctx, alpineID, client.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}

	wait := apiClient.ContainerWait(ctx, alpineID, client.ContainerWaitOptions{})
	select {
	case err = <-wait.Error:
		if err != nil {
			panic(err)
		}
	case <-wait.Result:
		fmt.Println("success!")
	}

	out, err := apiClient.ContainerLogs(ctx, alpineID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		panic(err)
	}
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}
