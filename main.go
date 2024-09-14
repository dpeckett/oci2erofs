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

package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/containerd/platforms"
	"github.com/dpeckett/archivefs/erofs"
	"github.com/dpeckett/archivefs/tarfs"
	"github.com/dpeckett/telemetry"
	"github.com/dpeckett/telemetry/v1alpha1"
	"github.com/dpeckett/uncompr"
	"github.com/immutos/oci2erofs/internal/constants"
	"github.com/immutos/oci2erofs/internal/docker"
	"github.com/immutos/oci2erofs/internal/oci"
	"github.com/immutos/oci2erofs/internal/util"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

func main() {
	persistentFlags := []cli.Flag{
		&cli.GenericFlag{
			Name:  "log-level",
			Usage: "Set the log verbosity level",
			Value: util.FromSlogLevel(slog.LevelInfo),
		},
	}

	initLogger := func(c *cli.Context) error {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: (*slog.Level)(c.Generic("log-level").(*util.LevelFlag)),
		})))

		return nil
	}

	// Collect anonymized usage statistics.
	var telemetryReporter *telemetry.Reporter

	initTelemetry := func(c *cli.Context) error {
		telemetryReporter = telemetry.NewReporter(c.Context, slog.Default(), telemetry.Configuration{
			BaseURL: constants.TelemetryURL,
			Tags:    []string{"oci2erofs"},
		})

		// Some basic system information.
		info := map[string]string{
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
			"num_cpu": fmt.Sprintf("%d", runtime.NumCPU()),
			"version": constants.Version,
		}

		telemetryReporter.ReportEvent(&v1alpha1.TelemetryEvent{
			Kind:   v1alpha1.TelemetryEventKindInfo,
			Name:   "ApplicationStart",
			Values: info,
		})

		return nil
	}

	shutdownTelemetry := func(c *cli.Context) error {
		if telemetryReporter == nil {
			return nil
		}

		telemetryReporter.ReportEvent(&v1alpha1.TelemetryEvent{
			Kind: v1alpha1.TelemetryEventKindInfo,
			Name: "ApplicationStop",
		})

		// Don't want to block the shutdown of the application for too long.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := telemetryReporter.Shutdown(ctx); err != nil {
			slog.Error("Failed to close telemetry reporter", slog.Any("error", err))
		}

		return nil
	}

	app := &cli.App{
		Name:      "oci2erofs",
		Usage:     "Convert OCI images into EROFS filesystems",
		Version:   constants.Version,
		ArgsUsage: "image_path",
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output EROFS filesystem image",
			},
			&cli.StringFlag{
				Name:    "ref",
				Aliases: []string{"r"},
				Usage:   "Image reference (if more than one image is present)",
			},
			&cli.StringFlag{
				Name:    "platform",
				Aliases: []string{"p"},
				Usage:   "Target platform in the 'os/arch' format",
			},
		}, persistentFlags...),
		Before: util.BeforeAll(initLogger, initTelemetry),
		After:  shutdownTelemetry,
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				slog.Error("Image path is required")
				return cli.ShowAppHelp(c)
			}
			imagePath := c.Args().First()

			tempDir, err := os.MkdirTemp("", "oci2erofs")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			defer os.RemoveAll(tempDir)

			// Is the image a directory or a tarball?
			fi, err := os.Stat(imagePath)
			if err != nil {
				return fmt.Errorf("failed to open image: %w", err)
			}

			var imageFS fs.FS
			if fi.IsDir() {
				imageFS = os.DirFS(imagePath)
			} else {
				imageFile, err := os.Open(imagePath)
				if err != nil {
					return fmt.Errorf("failed to open tarball: %w", err)
				}
				defer imageFile.Close()

				// Decompress the image if it is compressed.
				dr, err := uncompr.NewReader(imageFile)
				if err != nil {
					return fmt.Errorf("failed to create decompressing reader: %w", err)
				}
				defer dr.Close()

				// Create a temporary file to store the decompressed image.
				decompressedImageFile, err := os.OpenFile(
					filepath.Join(tempDir, filepath.Base(imagePath)+".tar"), os.O_CREATE|os.O_RDWR, 0o644)
				if err != nil {
					_ = imageFile.Close()
					return fmt.Errorf("failed to create temporary tar file: %w", err)
				}
				defer decompressedImageFile.Close()

				if _, err := io.Copy(decompressedImageFile, dr); err != nil {
					return fmt.Errorf("failed to decompress image: %w", err)
				}

				imageFS, err = tarfs.Open(decompressedImageFile)
				if err != nil {
					return fmt.Errorf("failed to open tarball: %w", err)
				}
			}

			var platform *ocispecs.Platform
			if c.String("platform") != "" {
				parsed, err := platforms.Parse(c.String("platform"))
				if err != nil {
					return fmt.Errorf("failed to parse platform: %w", err)
				}
				platform = &parsed
			}

			// Determine if the image is a Docker or OCI image.
			var dockerArchive, ociArchive bool
			if _, err := imageFS.Open("manifest.json"); err == nil {
				dockerArchive = true
			}
			if !dockerArchive {
				if _, err := imageFS.Open("oci-layout"); err == nil {
					ociArchive = true
				}
			}
			if !dockerArchive && !ociArchive {
				return fmt.Errorf("image is not a valid OCI or Docker image")
			}

			var rootFS fs.FS
			var closeAll func() error
			if dockerArchive {
				rootFS, closeAll, err = docker.LoadImage(tempDir, imageFS, c.String("ref"), platform)
				if err != nil {
					return fmt.Errorf("failed to load Docker image: %w", err)
				}
			} else {
				rootFS, closeAll, err = oci.LoadImage(tempDir, imageFS, c.String("ref"), platform)
				if err != nil {
					return fmt.Errorf("failed to load OCI image: %w", err)
				}
			}
			defer func() {
				if err := closeAll(); err != nil {
					slog.Warn("Failed to close image layers", slog.Any("error", err))
				}
			}()

			outputPath := c.String("output")
			if outputPath == "" {
				if fi.IsDir() {
					outputPath = filepath.Base(imagePath) + ".erofs"
				} else {
					outputPath = strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath)) + ".erofs"
				}
			}

			// Remove the output file if it already exists.
			_ = os.Remove(outputPath)

			outputFile, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer outputFile.Close()

			if err := erofs.Create(outputFile, rootFS); err != nil {
				return fmt.Errorf("failed to create EROFS filesystem: %w", err)
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		slog.Error("Error", slog.Any("error", err))
		os.Exit(1)
	}
}
