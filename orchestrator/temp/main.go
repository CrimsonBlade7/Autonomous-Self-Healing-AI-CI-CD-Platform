package main

import (
	"context"
	"fmt"
	_ "io"
	_ "os"

	_ "github.com/moby/moby/api/pkg/stdcopy"
	_ "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func main() {
	var err error
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return
	}
	defer apiClient.Close()

	// pull some test images
	ctx := context.Background()
	// var reader client.ImagePullResponse
	// reader, err = apiClient.ImagePull(ctx, "docker.io/library/alpine:latest", client.ImagePullOptions{})
	// if err != nil {
	// 	panic(err)
	// }
	// io.Copy(os.Stdout, reader)

	// reader, err = apiClient.ImagePull(ctx, "docker.io/library/postgres:latest", client.ImagePullOptions{})
	// if err != nil {
	// 	panic(err)
	// }
	// io.Copy(os.Stdout, reader)

	// reader, err = apiClient.ImagePull(ctx, "docker.io/library/redis:latest", client.ImagePullOptions{})
	// if err != nil {
	// 	panic(err)
	// }
	// io.Copy(os.Stdout, reader)

	// alpineContainer, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
	// 	Image: "alpine:latest",
	// 	Config: &container.Config{
	// 		Cmd: []string{"echo", "alpine starting"},
	// 	},
	// 	Name: "alpine-container",
	// })
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Alpine container created: %s\n", alpineContainer.ID)

	// postgresContainer, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
	// 	Image: "postgres:latest",
	// 	Config: &container.Config{
	// 		Cmd: []string{"echo", "postgres starting"},
	// 	},
	// 	Name: "postgres-container",
	// })
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Postgres container created: %s\n", postgresContainer.ID)

	// redisContainer, err := apiClient.ContainerCreate(ctx,
	// 	client.ContainerCreateOptions{
	// 		Image: "redis:latest",
	// 		Config: &container.Config{
	// 			Cmd: []string{"echo", "postgres starting"},
	// 		},
	// 		Name: "redis-container",
	// 	})
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Redis container created: %s\n", redisContainer.ID)

	// can run containers here

	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Number of containers: %v\n", len(containers.Items))

	for i, container := range containers.Items {
		fmt.Printf("Container %v: %v\n", i, container.ID)
	}

}
