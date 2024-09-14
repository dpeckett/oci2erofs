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

package docker_test

import (
	"os"
	"testing"

	"github.com/dpeckett/archivefs/tarfs"
	"github.com/immutos/oci2erofs/internal/docker"
	"github.com/immutos/oci2erofs/internal/util"
	"github.com/stretchr/testify/require"
)

func TestLoadImage(t *testing.T) {
	ref := "docker.io/tianon/toybox:0.8.11"

	imageFile, err := os.Open("testdata/toybox.tar")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, imageFile.Close())
	})

	imageFS, err := tarfs.Open(imageFile)
	require.NoError(t, err)

	rootFS, closeAll, err := docker.LoadImage(t.TempDir(), imageFS, ref, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closeAll())
	})

	h, err := util.HashFS(rootFS)
	require.NoError(t, err)

	require.Equal(t, "h1:J674XgTpeE71MnUmfTovrhKIlFnWOa8rctDF6SL/Kzg=", h)
}
