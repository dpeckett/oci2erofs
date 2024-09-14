// SPDX-License-Identifier: AGPL-3.0-or-later
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpeckett/archivefs/tarfs"
	"github.com/dpeckett/uncompr"
	"github.com/immutos/oci2erofs/internal/overlayfs"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

// LoadImage loads a Docker image from the given imageFS, ref, and platform.
// It returns an overlayfs.FS of the image's root filesystem, a function to
// close the image, and an error if any.
func LoadImage(tempDir string, imageFS fs.FS, ref string, platform *ocispecs.Platform) (fs.FS, func() error, error) {
	config, err := configForRef(imageFS, ref, platform)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get image config: %w", err)
	}

	var layers []fs.FS
	var closers []func() error

	for _, layerDescriptor := range config.RootFS.DiffIDs {
		layerPath := filepath.Join(strings.TrimPrefix(layerDescriptor, "sha256:") + ".tar")

		layer, close, err := loadLayer(tempDir, imageFS, layerPath)
		if err != nil {
			layerPath += ".gz"

			var err2 error
			layer, close, err2 = loadLayer(tempDir, imageFS, layerPath)
			if err2 != nil {
				return nil, nil, fmt.Errorf("failed to load layer: %w", err)
			}
		}

		layers = append(layers, layer)
		closers = append(closers, close)
	}

	closeAll := func() error {
		for _, close := range closers {
			if err := close(); err != nil {
				return err
			}
		}
		return nil
	}

	rootFS, err := overlayfs.New(layers)
	if err != nil {
		_ = closeAll()
		return nil, nil, fmt.Errorf("failed to create overlayfs: %w", err)
	}

	return rootFS, closeAll, nil
}

func configForRef(imageFS fs.FS, ref string, platform *ocispecs.Platform) (*Config, error) {
	manifestFile, err := imageFS.Open("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest: %w", err)
	}
	defer manifestFile.Close()

	var manifests []Manifest
	if err := json.NewDecoder(manifestFile).Decode(&manifests); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	if len(manifests) == 0 {
		return nil, fmt.Errorf("no manifests found")
	}

	var manifest *Manifest
	if ref == "" {
		if len(manifests) > 1 {
			return nil, fmt.Errorf("multiple manifests found, ref must be specified")
		}

		manifest = &manifests[0]
	} else {
		for _, m := range manifests {
			for _, tag := range m.RepoTags {
				if tag == ref {
					manifest = &m
					break
				}
			}
		}
	}
	if manifest == nil {
		return nil, fmt.Errorf("no manifest found for ref %s", ref)
	}

	configFile, err := imageFS.Open(manifest.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to open image config: %w", err)
	}
	defer configFile.Close()

	var config Config
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal image config: %w", err)
	}

	if platform != nil && (config.Architecture != platform.Architecture || config.OS != platform.OS) {
		return nil, fmt.Errorf("no manifest found for platform %s/%s", platform.Architecture, platform.OS)
	}

	return &config, nil
}

func loadLayer(tempDir string, imageFS fs.FS, layerPath string) (fs.FS, func() error, error) {
	f, err := imageFS.Open(layerPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open layer: %w", err)
	}
	defer f.Close()

	dr, err := uncompr.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create decompressing reader: %w", err)
	}
	defer dr.Close()

	decompressedLayerPath := filepath.Join(tempDir, filepath.Base(layerPath)+".tar")
	decompressedLayerFile, err := os.OpenFile(decompressedLayerPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary tar file: %w", err)
	}

	if _, err := io.Copy(decompressedLayerFile, dr); err != nil {
		return nil, nil, fmt.Errorf("failed to decompress layer: %w", err)
	}

	fsys, err := tarfs.Open(decompressedLayerFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open decompressed layer: %w", err)
	}

	return fsys, decompressedLayerFile.Close, nil
}
