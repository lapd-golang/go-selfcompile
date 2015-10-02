package selfcompile

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

const srcdir = "_self"
const vendordir = "_vendor"
const pluginfile = "plugin_selfcompile.go"
const tmpprefix = "go-selfcompile"

var errRestoreAssets = errors.New("missing RestoreAssets")

type RestoreAssets func(dir, name string) error

// SelfCompile provides controls for registering new plugins and re-compiling
// the binary.
type SelfCompile struct {
	pkg     string // Package to use for plugins (empty will default to "main")
	plugins []string

	// Main package source URL to install on recompile (if not bundled).
	Install string

	// Automatically call SelfCompile.Cleanup() after Compile() is done.
	AutoCleanup bool

	// Parameters used to setup the temporary workdir.
	Prefix    string // Prefix for TempDir, used to stage recompiling assets.
	Root      string // Root of TempDir (empty will use OS default).
	workdir   string // Full path to the work dir once it has been created.
	srcdir    string // Full path to source dir of our package.
	vendordir string // Full path to GOPATH dir for our dependencies.

	// RestoreAssets is the function generated by bindata to restore the assets
	// recursively within a given directory.
	RestoreAssets RestoreAssets
}

// Plugin registers a new plugin to self-compile
func (c *SelfCompile) Plugin(p string) {
	c.plugins = append(c.plugins, p)
}

// Compile will recompile the program's source with the registered plugins.
func (c *SelfCompile) Compile() (err error) {
	err = c.setup()
	if err != nil {
		return
	}
	if c.AutoCleanup {
		defer func() {
			err = combineErrors(c.Cleanup(), err)
		}()
	}

	if c.Install == "" {
		// TODO: Handle bundled source if c.Install is not defined
		err = errors.New("not implemented: Bundled source, must specify Install target.")
		return
	}

	logger.Println("Compiling workdir:", c.workdir)

	err = c.goRun("get", c.Install)
	return
}

func (c *SelfCompile) goRun(args ...string) error {
	// FIXME: Default to env = os.Environ()?
	env := []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		fmt.Sprintf("GOROOT=%s", c.workdir),
		fmt.Sprintf("GOPATH=%s", c.vendordir),
	}

	cmd := exec.Cmd{
		Path: filepath.Join(c.workdir, "bin", "go"),
		Args: append([]string{"go"}, args...),
		Env:  env,
		Dir:  c.workdir,

		// TODO: Eat outputs and do something with them?
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return cmd.Run()
}

// stubPlugins will generate import stub files for the registered plugins.
func (c *SelfCompile) stubPlugins() error {
	// TODO: Use a mock fs to test: https://talks.golang.org/2012/10things.slide#8
	path := filepath.Join(c.srcdir, pluginfile)
	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fd.Close()

	w := bufio.NewWriter(fd)
	defer w.Flush()

	p := plugin{
		Package: c.pkg,
		Imports: c.plugins,
	}
	_, err = p.WriteTo(w)
	return err
}

// setup will create a fresh temporary directory and inflate all the binary
// data with the appropriate layout inside of it.
func (c *SelfCompile) setup() error {
	var err error
	if c.RestoreAssets == nil {
		return errRestoreAssets
	}

	prefix := tmpprefix
	if c.Prefix != "" {
		prefix = c.Prefix
	}
	c.workdir, err = ioutil.TempDir(c.Root, prefix)
	if err != nil {
		return err
	}
	logger.Printf("Initializing workdir: %s", c.workdir)

	c.vendordir = filepath.Join(c.workdir, vendordir)
	if c.Install == "" {
		// Assume we embedded the source
		c.srcdir = filepath.Join(c.workdir, srcdir)
	} else {
		c.srcdir = filepath.Join(c.vendordir, "src", c.Install)
	}

	// Restore all the assets recursively
	err = c.RestoreAssets(c.workdir, "")
	if err != nil {
		return err
	}

	if c.Install != "" {
		// Fetch source
		err := c.goRun("get", "-d", c.Install)
		if err != nil {
			return err
		}
	}

	// Generate plugin stubs in srcdir
	err = c.stubPlugins()
	if err != nil {
		return err
	}

	return nil
}

// Cleanup will delete any temporary files created for the workdir, good idea to
// call this as a defer after calling setup().
func (c *SelfCompile) Cleanup() error {
	if c.workdir == "" {
		// No workdir setup, nothing to clean up.
		return nil
	}
	logger.Printf("Cleaning up: %s", c.workdir)
	return os.RemoveAll(c.workdir)
}
