package selfcompile

import (
	"bufio"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

const srcdir = "_src"
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
func (c *SelfCompile) Compile() error {
	err := c.setup()
	if err != nil {
		return err
	}
	//defer c.cleanup() // TODO: Handle cleanup error
	// TODO: ...
	return nil
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
	c.srcdir = filepath.Join(c.workdir, srcdir)
	c.vendordir = filepath.Join(c.workdir, vendordir)

	// Restore all the assets recursively
	err = c.RestoreAssets(c.workdir, "")
	if err != nil {
		return err
	}

	// Generate plugin stubs
	err = c.stubPlugins()
	if err != nil {
		return err
	}

	return nil
}

// cleanup will delete any temporary files created for the workdir, good idea to
// call this as a defer after calling setup().
func (c *SelfCompile) cleanup() error {
	if c.workdir == "" {
		// No workdir setup, nothing to clean up.
		return nil
	}
	return os.RemoveAll(c.workdir)
}
