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

package overlayfs_test

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpeckett/archivefs/tarfs"
	"github.com/dpeckett/oci2erofs/internal/overlayfs"
	"github.com/dpeckett/uncompr"
	"github.com/rogpeppe/go-internal/dirhash"
	"github.com/stretchr/testify/require"
)

func TestOverlayFS(t *testing.T) {
	tempDir := t.TempDir()

	layerDigests := []string{
		"sha256:dc5fb67ee053f92fbe7e6e0218b489a4e61fa7302d5fe7de0792dc8c96290089",
		"sha256:45c7500cb6d4654528c454f08a66cae0a8006ed11b09ede60bc9b7755758333f",
		"sha256:43e4e9299522c0bc75d6d60eefa67e280aa1a1f065e835df9ad80dc3c689b806",
		"sha256:24ea87a8a4997700762bf25f254695bdafb187a68e54c3a0b1860d67cc839beb",
		"sha256:8d6878e5e3bfe70d09f005a5e658406222050fc045aa86fb51784ba61a74f5c2",
		"sha256:04b2bfec20d6d4ee1a56d7ddc70a5d1d929f72a913eea67ae792e78df58a25ba",
		"sha256:46dee9d05bd1b1b6d0d2ee899f636d9ae029e37d4266a6cdb4dba54249327120",
		"sha256:3162c1098f3291379e422f7e54703504ca44d39ab133ecff51d9ff6053d14e98",
		"sha256:7172bbbb95fa2d0b3785ce6638c28393d846359f1e5d65c0fa8b7af194e1cf2d",
	}

	layers := make([]fs.FS, len(layerDigests))
	for i, digest := range layerDigests {
		parts := strings.Split(digest, ":")
		path := filepath.Join("testdata/image/blobs", parts[0], parts[1])

		compressedLayer, err := os.Open(path)
		require.NoError(t, err)

		dr, err := uncompr.NewReader(compressedLayer)
		if err != nil {
			_ = compressedLayer.Close()
			require.NoError(t, err)
		}
		t.Cleanup(func() {
			require.NoError(t, dr.Close())
		})

		decompressedLayer, err := os.Create(filepath.Join(tempDir, digest+".tar"))
		if err != nil {
			_ = compressedLayer.Close()
			require.NoError(t, err)
		}

		_, err = io.Copy(decompressedLayer, dr)
		_ = compressedLayer.Close()
		require.NoError(t, err)

		layers[i], err = tarfs.Open(decompressedLayer)
		require.NoError(t, err)
	}

	fsys, err := overlayfs.New(layers)
	require.NoError(t, err)

	t.Run("Open", func(t *testing.T) {
		t.Run("Regular", func(t *testing.T) {
			f, err := fsys.Open("foo/a")
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, f.Close())
			})

			content, err := io.ReadAll(f)
			require.NoError(t, err)

			require.Equal(t, "hello world\n", string(content))

			f, err = fsys.Open("foo/b")
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, f.Close())
			})

			content, err = io.ReadAll(f)
			require.NoError(t, err)

			require.Equal(t, "quuz corge\n", string(content))
		})

		t.Run("Deleted", func(t *testing.T) {
			_, err := fsys.Open("foo/c")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})

		t.Run("Not Exist", func(t *testing.T) {
			_, err := fsys.Open("bang")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})

		t.Run("Symlink", func(t *testing.T) {
			t.Run("Absolute", func(t *testing.T) {
				f, err := fsys.Open("foo/baz/a")
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, f.Close())
				})

				content, err := io.ReadAll(f)
				require.NoError(t, err)

				require.Equal(t, "hello world\n", string(content))
			})

			t.Run("Relative", func(t *testing.T) {
				f, err := fsys.Open("bin/ls")
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, f.Close())
				})

				fi, err := f.Stat()
				require.NoError(t, err)

				require.True(t, fi.Mode().IsRegular())
			})
		})

		t.Run("Cross Layer Symlink", func(t *testing.T) {
			f, err := fsys.Open("foo/bar/b")
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, f.Close())
			})

			content, err := io.ReadAll(f)
			require.NoError(t, err)

			require.Equal(t, "foo bar baz\n", string(content))
		})
	})

	t.Run("ReadDir", func(t *testing.T) {
		t.Run("Regular", func(t *testing.T) {
			entries, err := fs.ReadDir(fsys, "foo")
			require.NoError(t, err)

			require.Len(t, entries, 4)

			require.Equal(t, "a", entries[0].Name())
			require.True(t, entries[0].Type().IsRegular())

			fi, err := entries[0].Info()
			require.NoError(t, err)
			require.Equal(t, int64(12), fi.Size())

			require.Equal(t, "b", entries[1].Name())
			require.True(t, entries[1].Type().IsRegular())

			fi, err = entries[1].Info()
			require.NoError(t, err)
			require.Equal(t, int64(11), fi.Size())
		})

		t.Run("Not Exist", func(t *testing.T) {
			_, err := fs.ReadDir(fsys, "bang")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})

		t.Run("Symlink", func(t *testing.T) {
			entries, err := fs.ReadDir(fsys, "foo/baz")
			require.NoError(t, err)

			require.Len(t, entries, 1)

			require.Equal(t, "a", entries[0].Name())
			require.True(t, entries[0].Type().IsRegular())

			fi, err := entries[0].Info()
			require.NoError(t, err)
			require.Equal(t, int64(12), fi.Size())
		})

		t.Run("Cross Layer Symlink", func(t *testing.T) {
			entries, err := fs.ReadDir(fsys, "foo/bar")
			require.NoError(t, err)

			require.Len(t, entries, 1)

			require.Equal(t, "b", entries[0].Name())
			require.True(t, entries[0].Type().IsRegular())

			fi, err := entries[0].Info()
			require.NoError(t, err)
			require.Equal(t, int64(12), fi.Size())
		})

		t.Run("Root", func(t *testing.T) {
			entries, err := fs.ReadDir(fsys, ".")
			require.NoError(t, err)

			require.Len(t, entries, 18)

			var names []string
			for _, entry := range entries {
				names = append(names, entry.Name())
			}

			expected := []string{"bar", "baz", "bin", "dev", "etc", "foo", "home",
				"init", "lib", "mnt", "proc", "root", "run", "sbin", "sys", "tmp",
				"usr", "var"}

			require.Equal(t, expected, names)
		})
	})

	t.Run("Stat", func(t *testing.T) {
		t.Run("Regular", func(t *testing.T) {
			fi, err := fsys.Stat("foo/a")
			require.NoError(t, err)

			require.True(t, fi.Mode().IsRegular())
			require.Equal(t, int64(12), fi.Size())
		})

		t.Run("Deleted", func(t *testing.T) {
			_, err := fsys.Stat("bar/c")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})

		t.Run("Symlink", func(t *testing.T) {
			fi, err := fsys.Stat("foo/baz/a")
			require.NoError(t, err)

			require.True(t, fi.Mode().IsRegular())
			require.Equal(t, int64(12), fi.Size())
		})

		t.Run("Cross Layer Symlink", func(t *testing.T) {
			fi, err := fsys.Stat("foo/bar/b")
			require.NoError(t, err)

			require.True(t, fi.Mode().IsRegular())
			require.Equal(t, int64(12), fi.Size())
		})
	})

	t.Run("ReadLink", func(t *testing.T) {
		t.Run("Regular", func(t *testing.T) {
			_, err := fsys.ReadLink("foo/a")
			require.ErrorIs(t, err, fs.ErrInvalid)
		})

		t.Run("Deleted", func(t *testing.T) {
			_, err := fsys.ReadLink("bar/c")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})

		t.Run("Symlink", func(t *testing.T) {
			t.Run("Absolute", func(t *testing.T) {
				target, err := fsys.ReadLink("foo/baz")
				require.NoError(t, err)

				require.Equal(t, "/baz", target)
			})

			t.Run("Relative", func(t *testing.T) {
				target, err := fsys.ReadLink("bin")
				require.NoError(t, err)

				require.Equal(t, "usr/bin", target)
			})
		})

		t.Run("Cross Layer Symlink", func(t *testing.T) {
			target, err := fsys.ReadLink("foo/bar")
			require.NoError(t, err)

			require.Equal(t, "/bar", target)
		})
	})

	t.Run("StatLink", func(t *testing.T) {
		t.Run("Regular", func(t *testing.T) {
			fi, err := fsys.StatLink("foo/a")
			require.NoError(t, err)

			require.True(t, fi.Mode().IsRegular())
			require.Equal(t, int64(12), fi.Size())
		})

		t.Run("Deleted", func(t *testing.T) {
			_, err := fsys.StatLink("bar/c")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})

		t.Run("Symlink", func(t *testing.T) {
			fi, err := fsys.StatLink("foo/baz")
			require.NoError(t, err)

			require.True(t, fi.Mode()&fs.ModeSymlink != 0)
		})

		t.Run("Cross Layer Symlink", func(t *testing.T) {
			fi, err := fsys.StatLink("foo/bar")
			require.NoError(t, err)

			require.True(t, fi.Mode()&fs.ModeSymlink != 0)
		})
	})

	t.Run("Dirhash", func(t *testing.T) {
		var files []string
		err := fs.WalkDir(fsys, ".", func(file string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
				return nil
			}

			files = append(files, filepath.ToSlash(file))
			return nil
		})
		require.NoError(t, err)

		h, err := dirhash.Hash1(files, func(name string) (io.ReadCloser, error) {
			return fsys.Open(name)
		})
		require.NoError(t, err)

		require.Equal(t, "h1:hJbAbj8GzqpjzKJ7vPyenrzI/QB2YfM5RtMYnVrwiSo=", h)
	})
}
