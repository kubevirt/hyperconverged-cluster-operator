package images

import (
	"context"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
)

// GetArches returns a comma-separated list of architectures for the given image URL.
func GetArches(ctx context.Context, url string) (string, error) {
	url = strings.TrimPrefix(url, "docker:")
	if !strings.HasPrefix(url, "//") {
		url = "//" + url
	}

	imgRef, err := docker.ParseReference(url)
	if err != nil {
		return "", err
	}

	sysCtx := &types.SystemContext{}
	source, err := imgRef.NewImageSource(ctx, sysCtx)
	if err != nil {
		return "", err
	}
	defer source.Close()

	rawManifest, mimeType, err := source.GetManifest(ctx, nil)

	if err != nil {
		return "", err
	}

	if !manifest.MIMETypeIsMultiImage(mimeType) {
		return "", nil
	}

	m, err := manifest.ListFromBlob(rawManifest, mimeType)
	if err != nil {
		return "", err
	}

	arches, err := buildArchList(m)
	if err != nil {
		return "", err
	}

	return arches, nil
}

func buildArchList(m manifest.List) (string, error) {
	var arches []string
	for _, digest := range m.Instances() {
		instance, err := m.Instance(digest)
		if err != nil {
			return "", err
		}

		if platform := instance.ReadOnly.Platform; platform != nil {
			arches = append(arches, platform.Architecture)
		}
	}

	return strings.Join(arches, ","), nil
}
