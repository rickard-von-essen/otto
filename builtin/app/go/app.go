package goapp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/otto/app"
	"github.com/hashicorp/otto/directory"
	"github.com/hashicorp/otto/helper/bindata"
	"github.com/hashicorp/otto/helper/packer"
	"github.com/hashicorp/otto/helper/vagrant"
)

//go:generate go-bindata -pkg=goapp -nomemcopy ./data/...

// App is an implementation of app.App
type App struct{}

func (a *App) Compile(ctx *app.Context) (*app.CompileResult, error) {
	data := &bindata.Data{
		Asset:    Asset,
		AssetDir: AssetDir,
		Context: map[string]interface{}{
			"name":          ctx.Appfile.Application.Name,
			"dev_fragments": ctx.DevDepFragments,
			"path": map[string]string{
				"cache":    ctx.CacheDir,
				"compiled": ctx.Dir,
				"working":  filepath.Dir(ctx.Appfile.Path),
			},
		},
	}

	// Copy all the common files
	if err := data.CopyDir(ctx.Dir, "data/common"); err != nil {
		return nil, err
	}

	// Copy the infrastructure specific files
	prefix := fmt.Sprintf("data/%s-%s", ctx.Tuple.Infra, ctx.Tuple.InfraFlavor)
	if err := data.CopyDir(ctx.Dir, prefix); err != nil {
		return nil, err
	}

	return &app.CompileResult{
		DevDepFragmentPath: filepath.Join(ctx.Dir, "dev-dep/build/Vagrantfile.fragment"),
	}, nil
}

func (a *App) Build(ctx *app.Context) error {
	// Get the infrastructure state
	infra, err := ctx.Directory.GetInfra(directory.InfraId(
		ctx.Appfile.ActiveInfrastructure()))
	if err != nil {
		return err
	}

	if infra == nil || infra.State != directory.InfraStateReady {
		return fmt.Errorf(
			"Infrastructure for this application hasn't been built yet.\n" +
				"The build step requires this because the target infrastructure\n" +
				"as well as its final properties can affect the build process.\n" +
				"Please run `otto infra` to build the underlying infrastructure,\n" +
				"then run `otto build` again.")
	}

	ctx.Ui.Message(fmt.Sprintf("%#v", infra))

	p := &packer.Packer{Dir: ctx.Dir, Ui: ctx.Ui}
	return p.Execute("version")
}

func (a *App) Dev(ctx *app.Context) error {
	return vagrant.Dev(ctx, &vagrant.DevOptions{
		Instructions: strings.TrimSpace(devInstructions),
	})
}

func (a *App) DevDep(dst, src *app.Context) (*app.DevDep, error) {
	// For Go, we build a binary using Vagrant, extract that binary,
	// and setup a Vagrantfile fragment to copy that binary in plus
	// setup the scripts to start it on boot.
	src.Ui.Header(fmt.Sprintf(
		"Building the dev dependency for '%s'", src.Appfile.Application.Name))
	src.Ui.Message(
		"To ensure cross-platform compatibility, we'll use Vagrant to\n" +
			"build this application. This is slow, and in a lot of cases we\n" +
			"can do something faster. Future versions of Otto will detect and\n" +
			"do this. As long as the application doesn't change, Otto will\n" +
			"cache the results of this build.\n\n")
	err := vagrant.Build(src, &vagrant.BuildOptions{
		Dir:    filepath.Join(src.Dir, "dev-dep/build"),
		Script: "/otto/build.sh",
	})
	if err != nil {
		return nil, err
	}

	// Return the fragment path we have setup
	return &app.DevDep{
		Files: []string{"dev-dep-output"},
	}, nil
}

const devInstructions = `
A development environment has been created for writing a generic Go-based
application. For this development environment, Go is pre-installed. To
work on your project, edit files locally on your own machine. The file changes
will be synced to the development environment.

When you're ready to build your project, run 'otto dev ssh' to enter
the development environment. You'll be placed directly into the working
directory where you can run 'go get' and 'go build' as you normally would.
The GOPATH is already completely setup.
`
