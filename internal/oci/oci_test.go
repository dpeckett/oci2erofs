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

package oci_test

import (
	"os"
	"testing"

	"github.com/immutos/oci2erofs/internal/oci"
	"github.com/immutos/oci2erofs/internal/util"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestLoadImage(t *testing.T) {
	ref := "docker.io/tianon/toybox:0.8.11"

	t.Run("Single Arch", func(t *testing.T) {
		rootFS, closeAll, err := oci.LoadImage(t.TempDir(), os.DirFS("testdata/toybox"), ref, nil)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, closeAll())
		})

		h, err := util.HashFS(rootFS)
		require.NoError(t, err)

		require.Equal(t, "h1:J674XgTpeE71MnUmfTovrhKIlFnWOa8rctDF6SL/Kzg=", h)
	})

	t.Run("Multi Arch", func(t *testing.T) {
		t.Run("amd64", func(t *testing.T) {
			platform := ocispecs.Platform{
				Architecture: "amd64",
				OS:           "linux",
			}

			rootFS, closeAll, err := oci.LoadImage(t.TempDir(), os.DirFS("testdata/toybox-multiarch"), ref, &platform)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, closeAll())
			})

			h, err := util.HashFS(rootFS)
			require.NoError(t, err)

			require.Equal(t, "h1:J674XgTpeE71MnUmfTovrhKIlFnWOa8rctDF6SL/Kzg=", h)
		})

		t.Run("arm64", func(t *testing.T) {
			platform := ocispecs.Platform{
				Architecture: "arm64",
				OS:           "linux",
			}

			rootFS, closeAll, err := oci.LoadImage(t.TempDir(), os.DirFS("testdata/toybox-multiarch"), ref, &platform)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, closeAll())
			})

			h, err := util.HashFS(rootFS)
			require.NoError(t, err)

			require.Equal(t, "h1:vep4P8xi3jVOxfV9SWQjzrHUoAIDjgYEGJ+yIYeq2JQ=", h)
		})
	})
}
