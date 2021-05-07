// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package js

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/talos-systems/kres/internal/dag"
	"github.com/talos-systems/kres/internal/output/dockerfile"
	"github.com/talos-systems/kres/internal/output/dockerfile/step"
	"github.com/talos-systems/kres/internal/output/drone"
	"github.com/talos-systems/kres/internal/output/makefile"
	"github.com/talos-systems/kres/internal/output/template"
	"github.com/talos-systems/kres/internal/project/common"
	"github.com/talos-systems/kres/internal/project/js/templates"
	"github.com/talos-systems/kres/internal/project/meta"
)

// Toolchain provides node js runtime and common utilities.
type Toolchain struct {
	dag.BaseNode

	meta *meta.Options

	sourceDir string
	Version   string
	Image     string
}

// NewToolchain builds Toolchain with default values.
func NewToolchain(meta *meta.Options, sourceDir string) *Toolchain {
	toolchain := &Toolchain{
		BaseNode: dag.NewBaseNode("js"),

		meta:      meta,
		sourceDir: sourceDir,

		Version: "15.14.0-alpine3.13",
	}

	meta.BuildArgs = append(meta.BuildArgs, "JS_TOOLCHAIN")
	meta.SourceFiles = append(meta.SourceFiles, ".babelrc", ".tsconfig")

	return toolchain
}

// CompileTemplates implements template.Compiler.
func (toolchain *Toolchain) CompileTemplates(output *template.Output) error {
	output.Define(".babelrc", templates.Babel).
		PreamblePrefix("// ")

	output.Define(".tsconfig", templates.TSConfig).
		NoPreamble()

	return nil
}

func (toolchain *Toolchain) image() string {
	if toolchain.Image != "" {
		return toolchain.Image
	}

	return fmt.Sprintf("docker.io/node:%s", toolchain.Version)
}

// CompileMakefile implements makefile.Compiler.
func (toolchain *Toolchain) CompileMakefile(output *makefile.Output) error {
	output.VariableGroup(makefile.VariableGroupDocker).
		Variable(makefile.OverridableVariable("JS_TOOLCHAIN", toolchain.image()))

	output.Target("js").
		Description("Prepare js base toolchain.").
		Script("@$(MAKE) target-$@").
		Phony()

	return nil
}

// CompileDrone implements drone.Compiler.
func (toolchain *Toolchain) CompileDrone(output *drone.Output) error {
	output.Step(drone.MakeStep("js").
		DependsOn(dag.GatherMatchingInputNames(toolchain, dag.Implements((*drone.Compiler)(nil)))...),
	)

	return nil
}

// CompileDockerfile implements dockerfile.Compiler.
func (toolchain *Toolchain) CompileDockerfile(output *dockerfile.Output) error {
	output.Arg(step.Arg("JS_TOOLCHAIN"))

	toolchain.meta.NpmCachePath = "/src/node_modules"

	output.Stage("js-toolchain").
		Description("base toolchain image").
		From("${JS_TOOLCHAIN}").
		Step(step.Run("apk", "--update", "--no-cache", "add", "bash", "curl", "protoc", "protobuf-dev", "go")).
		Step(step.Copy("./go.mod", ".")).
		Step(step.Copy("./go.sum", ".")).
		Step(step.Env("GOPATH", toolchain.meta.GoPath))

	base := output.Stage("js").
		Description("tools and sources").
		From("js-toolchain").
		Step(step.WorkDir("/src"))

	if err := dag.WalkNode(toolchain, func(node dag.Node) error {
		if builder, ok := node.(common.ToolchainBuilder); ok {
			return builder.ToolchainBuild(base)
		}

		return nil
	}, nil, 1); err != nil {
		return err
	}

	base.Step(step.Copy(filepath.Join(toolchain.sourceDir, "package.json"), "./")).
		Step(step.Copy(filepath.Join(toolchain.sourceDir, "package-lock.json"), "./")).
		Step(step.Script("npm version ${VERSION}").
			MountCache(toolchain.meta.NpmCachePath)).
		Step(step.Script("npm install").
			MountCache(toolchain.meta.NpmCachePath)).
		Step(step.Copy(".eslintrc.yaml", "./")).
		Step(step.Copy(".babelrc", "./babel.config.js")).
		Step(step.Copy(".jestrc", "./jest.config.js")).
		Step(step.Copy(".tsconfig", "./tsconfig.json"))

	for _, directory := range toolchain.meta.JSDirectories {
		dest := strings.TrimLeft(directory, toolchain.sourceDir)

		base.Step(step.Copy("./"+directory, "./"+strings.Trim(dest, "/")))
	}

	for _, file := range toolchain.meta.JSSourceFiles {
		dest := filepath.Base(file)

		base.Step(step.Copy(file, "./"+dest))
	}

	return nil
}

// SkipAsMakefileDependency implements makefile.SkipAsMakefileDependency.
func (toolchain *Toolchain) SkipAsMakefileDependency() {
}
