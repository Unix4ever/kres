// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package js

import (
	"fmt"
	"path/filepath"

	"github.com/talos-systems/kres/internal/dag"
	"github.com/talos-systems/kres/internal/output/dockerfile"
	"github.com/talos-systems/kres/internal/output/dockerfile/step"
	"github.com/talos-systems/kres/internal/output/drone"
	"github.com/talos-systems/kres/internal/output/makefile"
	"github.com/talos-systems/kres/internal/output/template"
	"github.com/talos-systems/kres/internal/project/js/templates"
	"github.com/talos-systems/kres/internal/project/meta"
)

// Build produces binaries for Go programs.
type Build struct {
	dag.BaseNode

	embedFile string
	meta      *meta.Options
	artifacts []string
}

// NewBuild initializes Build.
func NewBuild(meta *meta.Options, name string) *Build {
	embedFile := fmt.Sprintf("internal/%s/%s.go", name, name)
	meta.SourceFiles = append(meta.SourceFiles, embedFile)

	return &Build{
		BaseNode:  dag.NewBaseNode(name),
		meta:      meta,
		artifacts: []string{},
		embedFile: embedFile,
	}
}

// CompileTemplates implements template.Compiler.
func (build *Build) CompileTemplates(output *template.Output) error {
	output.Define(build.embedFile, templates.GoEmbed).
		Params(map[string]string{
			"project": build.Name(),
		}).
		PreamblePrefix("// ").
		WithLicense()

	distDir := filepath.Join(
		filepath.Dir(build.embedFile),
		"dist",
		".gitkeep",
	)

	output.Define(distDir, "").NoPreamble()

	return nil
}

// CompileDockerfile implements dockerfile.Compiler.
func (build *Build) CompileDockerfile(output *dockerfile.Output) error {
	outputDir := fmt.Sprintf("/internal/%s/dist", build.Name())

	output.Stage(build.Name()).
		Description(fmt.Sprintf("builds %s", build.Name())).
		From("js").
		Step(step.Script("npm run build").
			MountCache(build.meta.NpmCachePath)).
		Step(step.Script("mkdir -p " + outputDir)).
		Step(step.Script("cp -rf ./dist/* " + outputDir))

	build.artifacts = []string{outputDir}

	return nil
}

// CompileDrone implements drone.Compiler.
func (build *Build) CompileDrone(output *drone.Output) error {
	output.Step(drone.MakeStep(build.Name()).DependsOn(dag.GatherMatchingInputNames(build, dag.Implements((*drone.Compiler)(nil)))...))

	return nil
}

// CompileMakefile implements makefile.Compiler.
func (build *Build) CompileMakefile(output *makefile.Output) error {
	output.Target(fmt.Sprintf("$(ARTIFACTS)/%s-js", build.Name())).
		Script(fmt.Sprintf("@$(MAKE) local-%s DEST=$(ARTIFACTS)", build.Name())).
		Phony()

	output.Target(build.Name()).
		Description(fmt.Sprintf("Builds js release for %s.", build.Name())).
		Depends(fmt.Sprintf("$(ARTIFACTS)/%s-js", build.Name())).
		Phony()

	return nil
}

// GetArtifacts implements dockerfile.Generator.
func (build *Build) GetArtifacts() []string {
	return build.artifacts
}
