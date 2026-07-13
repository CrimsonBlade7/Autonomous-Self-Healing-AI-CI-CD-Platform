package main

import (
	"context"
	"fmt"
	"io"
	"os"
	_ "slices"
	"strings"

	_ "github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	_ "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
)

var imagesNames = []string{"alpine", "postgres", "redis"}
var contsItems []container.Summary

func getContainerName(ctx context.Context, c *client.Client, id string) string {
	contInfo, err := c.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		panic(err)
	}
	return strings.TrimPrefix(contInfo.Container.Name, "/")
}

// Reloads the contsItems slice
func reloadContainerSlice(ctx context.Context, c *client.Client) {
	contsRes, err := c.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}
	contsItems = contsRes.Items
}

func main() {
	var err error
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return
	}
	defer apiClient.Close()

	ctx := context.Background()
	reloadContainerSlice(ctx, apiClient)

	// pull some test images
	var reader client.ImagePullResponse
	for _, name := range imagesNames {
		// pulling images based on imageNames
		reader, err = apiClient.ImagePull(ctx, fmt.Sprintf("docker.io/library/%s:latest", name), client.ImagePullOptions{})
		if err != nil {
			panic(err)
		}
		io.Copy(os.Stdout, reader)

		// creating containers for each image in imageNames without duplicates
		duplicate := false
		for _, contInfo := range contsItems {
			if name == getContainerName(ctx, apiClient, contInfo.ID) {
				duplicate = true
				break
			}
		}
		if !duplicate {
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

			contName := getContainerName(ctx, apiClient, newCont.ID)
			fmt.Printf("Name: %s | ID: %s\n", contName, newCont.ID)
		}
	}

	// can run containers here

	reloadContainerSlice(ctx, apiClient)

	fmt.Printf("Number of containers: %v\n", len(contsItems))

	for _, cont := range contsItems {
		fmt.Printf("%s container ID: %s\n", getContainerName(ctx, apiClient, cont.ID), cont.ID)
	}

}
