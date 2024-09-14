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

package util

import (
	"io"
	"io/fs"
	"path/filepath"

	"github.com/rogpeppe/go-internal/dirhash"
)

func HashFS(fsys fs.FS) (string, error) {
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
	if err != nil {
		return "", err
	}

	h, err := dirhash.Hash1(files, func(name string) (io.ReadCloser, error) {
		return fsys.Open(name)
	})
	if err != nil {
		return "", err
	}

	return h, nil
}
