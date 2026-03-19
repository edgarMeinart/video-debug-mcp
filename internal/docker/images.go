package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// ImageManager handles Docker image availability checks and pulls.
type ImageManager struct {
	cli *client.Client
}

// NewImageManager returns an ImageManager backed by the given Docker client.
func NewImageManager(cli *client.Client) *ImageManager {
	return &ImageManager{cli: cli}
}

// Exists reports whether an image with the given name is present locally.
func (m *ImageManager) Exists(ctx context.Context, imageName string) bool {
	_, _, err := m.cli.ImageInspectWithRaw(ctx, imageName)
	return err == nil
}

// EnsureImage pulls imageName if it is not already present locally.
func (m *ImageManager) EnsureImage(ctx context.Context, imageName string) error {
	if m.Exists(ctx, imageName) {
		return nil
	}
	reader, err := m.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	// Drain the response so the pull completes.
	_, err = io.Copy(io.Discard, reader)
	return err
}

// ListAvailable partitions imageNames into those present locally and those missing.
func (m *ImageManager) ListAvailable(ctx context.Context, imageNames []string) (available, missing []string) {
	for _, name := range imageNames {
		if m.Exists(ctx, name) {
			available = append(available, name)
		} else {
			missing = append(missing, name)
		}
	}
	return available, missing
}
