package command

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/otto/appfile"
	"github.com/hashicorp/otto/otto"
	"github.com/mitchellh/cli"
)

const (
	// DefaultAppfile is the default filename for the Appfile
	DefaultAppfile = "Appfile"

	// DefaultOutputDir is the default directory for data output
	DefaultOutputDir = "otto"
)

// FlagSetFlags is an enum to define what flags are present in the
// default FlagSet returned by Meta.FlagSet
type FlagSetFlags uint

const (
	FlagSetNone    FlagSetFlags = 0
	FlagSetAppfile FlagSetFlags = iota
	FlagSetOutputDir
)

// Meta are the meta-options that are available on all or most commands.
type Meta struct {
	CoreConfig *otto.CoreConfig
	Ui         cli.Ui

	// These are fields set by flags
	flagAppfile   string
	flagOutputDir string
}

// Appfile loads the Appfile according to the path given by the
// -appfile flag.
func (m *Meta) Appfile() (*appfile.File, error) {
	// Get the path to where the Appfile lives
	path := m.flagAppfile
	if path == "" {
		path = "."
	}

	// Verify the path is valid
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf(
			"Error checking Appfile path: %s", err)
	}
	if fi.IsDir() {
		path = filepath.Join(path, DefaultAppfile)
	}

	// Load the appfile
	app, err := appfile.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"Error parsing Appfile: %s", err)
	}

	return app, nil
}

// Core returns the core for the given Appfile. The file where the
// Appfile was loaded from should be set in appfile.File.Path. This
// root appfile path will be used as the default output directory
// for Otto.
func (m *Meta) Core(f *appfile.File) (*otto.Core, error) {
	outputDir := DefaultOutputDir
	if m.flagOutputDir != "" {
		outputDir = m.flagOutputDir
	}

	config := *m.CoreConfig
	config.Appfile = f
	config.OutputDir = filepath.Join(filepath.Dir(f.Path), outputDir)

	return otto.NewCore(&config)
}

// FlagSet returns a FlagSet with the common flags that every
// command implements. The exact behavior of FlagSet can be configured
// using the flags as the second parameter.
func (m *Meta) FlagSet(n string, fs FlagSetFlags) *flag.FlagSet {
	f := flag.NewFlagSet(n, flag.ContinueOnError)

	if fs&FlagSetAppfile != 0 {
		f.StringVar(&m.flagAppfile, "appfile", "", "")
	}

	if fs&FlagSetOutputDir != 0 {
		f.StringVar(&m.flagOutputDir, "output", "", "")
	}

	// Create an io.Writer that writes to our Ui properly for errors.
	// This is kind of a hack, but it does the job. Basically: create
	// a pipe, use a scanner to break it into lines, and output each line
	// to the UI. Do this forever.
	errR, errW := io.Pipe()
	errScanner := bufio.NewScanner(errR)
	go func() {
		for errScanner.Scan() {
			m.Ui.Error(errScanner.Text())
		}
	}()
	f.SetOutput(errW)

	return f
}