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

package overlayfs

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dpeckett/archivefs"
)

const (
	whiteoutPrefix     = ".wh."
	opaqueWhiteoutName = ".wh..wh..opq"
)

var (
	_ fs.FS                = (*FS)(nil)
	_ fs.ReadDirFS         = (*FS)(nil)
	_ fs.StatFS            = (*FS)(nil)
	_ archivefs.ReadLinkFS = (*FS)(nil)
)

// FS is an overlay file system.
type FS struct {
	root dirent
}

// New creates a new overlay file system from the given layers.
func New(layers []fs.FS) (*FS, error) {
	root := dirent{
		layer:     layers[len(layers)-1],
		layerPath: ".",
	}

	for _, layer := range layers {
		err := fs.WalkDir(layer, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Eg. dangling symlinks.
				if errors.Is(err, fs.ErrNotExist) {
					return fs.SkipDir
				}

				return err
			}

			if path == "." {
				return nil
			}

			dir, err := resolve(&root, filepath.Dir(path))
			if err != nil {
				return fmt.Errorf("failed to resolve directory %q: %w", filepath.Dir(path), err)
			}

			if d.Name() == opaqueWhiteoutName {
				dir.children = nil
				return nil
			}

			if strings.HasPrefix(d.Name(), whiteoutPrefix) {
				dir.removeChild(strings.TrimPrefix(d.Name(), whiteoutPrefix))
				return nil
			}

			dir.addChild(&dirent{
				DirEntry:  d,
				layer:     layer,
				layerPath: path,
			})

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk layer: %w", err)
		}
	}

	return &FS{
		root: root,
	}, nil
}

func (fsys *FS) Open(name string) (fs.File, error) {
	d, err := resolve(&fsys.root, name)
	if err != nil {
		return nil, err
	}

	return d.layer.Open(d.layerPath)
}

func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	d, err := resolve(&fsys.root, name)
	if err != nil {
		return nil, err
	}

	var children []fs.DirEntry
	for _, child := range d.children {
		children = append(children, child)
	}

	slices.SortFunc(children, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return children, nil
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	d, err := resolve(&fsys.root, name)
	if err != nil {
		return nil, err
	}

	return fs.Stat(d.layer, d.layerPath)
}

func (fsys *FS) ReadLink(name string) (string, error) {
	d, err := resolve(&fsys.root, filepath.Dir(name))
	if err != nil {
		return "", err
	}

	d, found := d.findChild(filepath.Base(name))
	if !found {
		return "", fs.ErrNotExist
	}

	linkFS, ok := d.layer.(archivefs.ReadLinkFS)
	if !ok {
		return "", fmt.Errorf("layer does not support symbolic links: %w", fs.ErrInvalid)
	}

	return linkFS.ReadLink(name)
}

func (fsys *FS) StatLink(name string) (fs.FileInfo, error) {
	d, err := resolve(&fsys.root, filepath.Dir(name))
	if err != nil {
		return nil, err
	}

	d, found := d.findChild(filepath.Base(name))
	if !found {
		return nil, fs.ErrNotExist
	}

	linkFS, ok := d.layer.(archivefs.ReadLinkFS)
	if !ok {
		return nil, fmt.Errorf("layer does not support symbolic links: %w", fs.ErrInvalid)
	}

	return linkFS.StatLink(name)
}

// resolve resolves the given path to a dirent.
func resolve(root *dirent, name string) (*dirent, error) {
	d := root

	name = sanitizePath(name)
	if name == "" {
		return d, nil
	}

	for _, component := range strings.Split(name, "/") {
		var found bool
		d, found = d.findChild(component)
		if !found {
			return nil, fs.ErrNotExist
		}

		if d.Type()&fs.ModeSymlink != 0 {
			linkFS, ok := d.layer.(archivefs.ReadLinkFS)
			if !ok {
				return nil, fmt.Errorf("layer does not support symbolic links: %w", fs.ErrInvalid)
			}

			// Read the symlink target.
			target, err := linkFS.ReadLink(d.layerPath)
			if err != nil {
				return nil, err
			}

			// Resolve the target.
			if !filepath.IsAbs(target) && d.parent != nil {
				d, err = resolve(d.parent, target)
				if err != nil {
					return nil, err
				}
			} else {
				// The target is an absolute path or the dirent is the root dirent.
				d, err = resolve(root, target)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return d, nil
}

func sanitizePath(name string) string {
	return strings.TrimPrefix(strings.TrimPrefix(filepath.Clean(filepath.ToSlash(strings.TrimSpace(name))), "."), "/")
}

type dirent struct {
	fs.DirEntry
	layer     fs.FS
	layerPath string
	parent    *dirent
	children  map[string]*dirent
}

func (d *dirent) findChild(name string) (*dirent, bool) {
	c, ok := d.children[name]
	return c, ok
}

func (d *dirent) addChild(child *dirent) {
	if d.children == nil {
		d.children = make(map[string]*dirent)
	}

	c := *child
	c.parent = d

	// do we already have a child with the same name?
	if existing, ok := d.children[c.Name()]; ok {
		c.children = existing.children
	}

	d.children[c.Name()] = &c
}

func (d *dirent) removeChild(name string) {
	delete(d.children, name)
}
