// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package base

import (
	"golang.org/x/tools/go/packages"

	"entgo.io/ent/entc"
)

// DefaultConfig for loading Go base.
var DefaultConfig = &packages.Config{Mode: packages.NeedName}

// PkgPath returns the Go package name for given target path.
// Even if the existing path is not exist yet in the filesystem.
//
// If base.Config is nil, DefaultConfig will be used to load base.
func PkgPath(config *packages.Config, target string) (string, error) {
	if config == nil {
		config = DefaultConfig
	}
	return entc.PkgPath(config, target)
}
