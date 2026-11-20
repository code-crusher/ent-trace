// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package entc_test

import (
	"os"
	"path/filepath"
	"testing"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestLoadGraphTarget(t *testing.T) {
	// The default target directory (one level above the schema
	// path) is inferred from the schema location.
	g, err := entc.LoadGraph("./load/testdata/valid", &gen.Config{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cwd(t), "load", "testdata"), g.Config.Target)
	require.Equal(t, "entgo.io/ent/entc/load/testdata", g.Config.Package)

	// A custom target directory resolves the codegen package
	// from the target directory instead of the schema location.
	// See https://github.com/ent/ent/issues/4403.
	g, err = entc.LoadGraph("./load/testdata/valid", &gen.Config{
		Target: "./load/testdata/target",
	})
	require.NoError(t, err)
	require.Equal(t, "./load/testdata/target", g.Config.Target)
	require.Equal(t, "entgo.io/ent/entc/load/testdata/target", g.Config.Package)

	// An absolute target directory is supported as well.
	g, err = entc.LoadGraph("./load/testdata/valid", &gen.Config{
		Target: filepath.Join(cwd(t), "load", "testdata", "target"),
	})
	require.NoError(t, err)
	require.Equal(t, "entgo.io/ent/entc/load/testdata/target", g.Config.Package)

	// The schema path can be a package path as well. In this case,
	// the target is resolved relative to the working directory.
	g, err = entc.LoadGraph("entgo.io/ent/entc/load/testdata/valid", &gen.Config{
		Target: "./load/testdata/target",
	})
	require.NoError(t, err)
	require.Equal(t, "entgo.io/ent/entc/load/testdata/target", g.Config.Package)

	// An explicitly provided package always wins.
	g, err = entc.LoadGraph("./load/testdata/valid", &gen.Config{
		Target:  "./load/testdata/target",
		Package: "example.com/target",
	})
	require.NoError(t, err)
	require.Equal(t, "example.com/target", g.Config.Package)
}

func TestPkgPath(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod": "module golang.org/x\n",
		"x.go":   "package x",
		"y/y.go": "package y",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0644))
	}
	config := &packages.Config{Mode: packages.NeedName, Dir: root}
	pkgPath, err := entc.PkgPath(config, filepath.Join(root, "ent"))
	require.NoError(t, err)
	require.Equal(t, "golang.org/x/ent", pkgPath)

	config.Dir = filepath.Join(root, "y")
	pkgPath, err = entc.PkgPath(config, filepath.Join(root, "y", "ent"))
	require.NoError(t, err)
	require.Equal(t, "golang.org/x/y/ent", pkgPath)

	pkgPath, err = entc.PkgPath(config, filepath.Join(root, "y", "z", "ent"))
	require.NoError(t, err)
	require.Equal(t, "golang.org/x/y/z/ent", pkgPath)

	_, err = entc.PkgPath(config, filepath.Join(root, "z", "e", "n", "t"))
	require.Error(t, err)

	// A nil config resolves the package path of
	// targets within the current module.
	pkgPath, err = entc.PkgPath(nil, "./load/testdata/target")
	require.NoError(t, err)
	require.Equal(t, "entgo.io/ent/entc/load/testdata/target", pkgPath)

	_, err = entc.PkgPath(nil, "../nonexistent/path/target")
	require.Error(t, err)
}

func cwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return abs
}
