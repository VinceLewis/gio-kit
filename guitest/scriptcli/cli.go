// Package scriptcli provides the small, application-independent command layer
// used by compiled consumer test launchers. It does not discover applications,
// load plugins, start subprocesses, use a network, or control a device.
package scriptcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
)

var (
	ErrUsage       = errors.New("guitest/scriptcli: invalid command")
	ErrRegistry    = errors.New("guitest/scriptcli: registry rejected script")
	ErrApplication = errors.New("guitest/scriptcli: invalid application")
)

// Names is a closed set of consumer-owned fixture or probe names.
type Names map[string]struct{}

// Application is one application instance returned by an Opener.
//
// Driver is always closed by Run. Close is optional and is invoked exactly
// once afterwards for opener-owned resources that are not owned by Driver.
// Probes is the per-instance implementation of the names allowlisted in
// Config.Probes. Screenshot is optional and receives the step context.
type Application struct {
	Driver     *guitest.Driver
	Probes     map[string]script.ProbeFunc
	Screenshot script.ScreenshotFunc
	Close      func() error
}

// Opener constructs the consumer application for one already-validated,
// allowlisted fixture name. It may return a partial Application with an error;
// Run still closes every returned resource.
type Opener func(context.Context, string) (Application, error)

// Config is the complete consumer-specific boundary. Fixture and probe names
// are validated before Open is called, so scripts cannot discover or invoke
// undeclared application behavior.
type Config struct {
	Open           Opener
	Fixtures       Names
	Probes         Names
	DumpOptions    guitest.DumpOptions
	ArtifactLimits script.ArtifactLimits
}

// Run parses `run -script FILE -artifacts DIRECTORY`, opens the selected
// compiled application, and executes the script. All application and artifact
// cleanup errors are joined with the primary error.
func Run(ctx context.Context, args []string, config Config) (result script.Result, returnErr error) {
	if ctx == nil {
		return result, fmt.Errorf("%w: nil context", ErrUsage)
	}
	if err := validateConfig(config); err != nil {
		return result, err
	}
	scriptPath, artifactDirectory, err := parseRun(args)
	if err != nil {
		return result, err
	}
	document, err := decodeFile(scriptPath)
	if err != nil {
		return result, err
	}
	usedProbes, err := validateRegistry(document, config)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	application, openErr := config.Open(ctx, document.Fixture)
	defer func() {
		returnErr = errors.Join(returnErr, closeApplication(application))
	}()
	if openErr != nil {
		return result, openErr
	}
	if application.Driver == nil {
		return result, fmt.Errorf("%w: opener returned a nil driver", ErrApplication)
	}
	probes, err := selectProbes(usedProbes, application.Probes)
	if err != nil {
		return result, err
	}
	writer, err := script.NewArtifactWriter(artifactDirectory, script.ArtifactOptions{
		Limits: config.ArtifactLimits, Screenshot: application.Screenshot,
	})
	if err != nil {
		return result, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, writer.Abort())
	}()
	return script.Run(ctx, application.Driver, document, script.Options{
		Probes: probes, Artifacts: writer, DumpOptions: config.DumpOptions,
	})
}

func parseRun(args []string) (string, string, error) {
	if len(args) == 0 || args[0] != "run" {
		return "", "", fmt.Errorf("%w: expected run", ErrUsage)
	}
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	scriptPath := flags.String("script", "", "versioned script JSON file")
	artifacts := flags.String("artifacts", "", "private artifact output directory")
	if err := flags.Parse(args[1:]); err != nil {
		return "", "", fmt.Errorf("%w: flags", ErrUsage)
	}
	if *scriptPath == "" || *artifacts == "" || flags.NArg() != 0 {
		return "", "", fmt.Errorf("%w: run requires -script and -artifacts", ErrUsage)
	}
	return *scriptPath, *artifacts, nil
}

func decodeFile(path string) (document script.Script, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return document, err
	}
	document, decodeErr := script.Decode(file)
	return document, errors.Join(decodeErr, file.Close())
}

func validateConfig(config Config) error {
	if config.Open == nil {
		return fmt.Errorf("%w: nil opener", ErrApplication)
	}
	for name := range config.Fixtures {
		if !validName(name, true) {
			return fmt.Errorf("%w: invalid fixture registry", ErrRegistry)
		}
	}
	for name := range config.Probes {
		if !validName(name, false) {
			return fmt.Errorf("%w: invalid probe registry", ErrRegistry)
		}
	}
	return nil
}

func validateRegistry(document script.Script, config Config) (Names, error) {
	if _, ok := config.Fixtures[document.Fixture]; !ok {
		return nil, fmt.Errorf("%w: fixture is not registered", ErrRegistry)
	}
	used := make(Names)
	for _, step := range document.Steps {
		if step.Op != script.OpExpectProbe {
			continue
		}
		if _, ok := config.Probes[step.Probe]; !ok {
			return nil, fmt.Errorf("%w: probe is not registered", ErrRegistry)
		}
		used[step.Probe] = struct{}{}
	}
	return used, nil
}

func selectProbes(used Names, implementations map[string]script.ProbeFunc) (map[string]script.ProbeFunc, error) {
	selected := make(map[string]script.ProbeFunc, len(used))
	for name := range used {
		probe, ok := implementations[name]
		if !ok || probe == nil {
			return nil, fmt.Errorf("%w: opener omitted a registered probe", ErrApplication)
		}
		selected[name] = probe
	}
	return selected, nil
}

func closeApplication(application Application) error {
	var driverErr, applicationErr error
	if application.Driver != nil {
		driverErr = application.Driver.Close()
	}
	if application.Close != nil {
		applicationErr = application.Close()
	}
	return errors.Join(driverErr, applicationErr)
}

func validName(name string, allowEmpty bool) bool {
	return (allowEmpty || name != "") && utf8.ValidString(name) && len(name) <= script.DefaultMaxString
}
