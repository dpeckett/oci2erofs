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

import "time"

// Manifest represents the Docker image manifest, typically found in manifest.json.
type Manifest struct {
	SchemaVersion int      `json:"schemaVersion"`
	MediaType     string   `json:"mediaType,omitempty"`
	Config        string   `json:"Config"`
	RepoTags      []string `json:"RepoTags"`
	Layers        []string `json:"Layers"`
}

// Config represents the Docker image configuration, typically found in the config.json file.
type Config struct {
	Architecture string      `json:"architecture"`
	OS           string      `json:"os"`
	OSVersion    string      `json:"os.version,omitempty"`
	OSFeatures   []string    `json:"os.features,omitempty"`
	Variant      string      `json:"variant,omitempty"`
	Created      time.Time   `json:"created"`
	Config       ImageConfig `json:"config"`
	RootFS       RootFS      `json:"rootfs"`
	History      []History   `json:"history"`
}

// ImageConfig represents the image configuration section of the Docker image config.
type ImageConfig struct {
	User         string              `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	StopSignal   string              `json:"StopSignal,omitempty"`
	ArgsEscaped  bool                `json:"ArgsEscaped,omitempty"`
	Shell        []string            `json:"Shell,omitempty"`
}

// RootFS represents the root filesystem section of the Docker image config.
type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

// History represents the history section of the Docker image config, detailing each layer.
type History struct {
	Created    time.Time `json:"created"`
	CreatedBy  string    `json:"created_by,omitempty"`
	Author     string    `json:"author,omitempty"`
	Comment    string    `json:"comment,omitempty"`
	EmptyLayer bool      `json:"empty_layer,omitempty"`
}
