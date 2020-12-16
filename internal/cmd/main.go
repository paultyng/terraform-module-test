package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/hashicorp/terraform-exec/tfinstall"
)

func Run(args []string) error {
	ctx := context.Background()

	// d := glint.New()

	// d.Append(
	// 	glint.Text("Looking for root modules..."),
	// )
	// d.Append(
	// 	glint.Style(
	// 		glint.TextFunc(func(rows, cols uint) string {
	// 			return fmt.Sprintf("%d tests passed", 5)
	// 		}),
	// 		glint.Color("green"),
	// 	),
	// )
	// d.Render(context.Background())
	var tfbins []string
	{
		// TODO: support version matrix, etc.
		tfbin, err := tfinstall.Find(ctx, tfinstall.LookPath(), tfinstall.LatestVersion("", false))
		if err != nil {
			return fmt.Errorf("unable to find Terraform binary: %w", err)
		}

		tfbins = append(tfbins, tfbin)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to determine work dir: %w", err)
	}

	// TODO: allow explicit
	var testDirs []string

	if testDirs == nil {
		candidates, err := findRootModules(wd)
		if err != nil {
			return fmt.Errorf("unable to identify test root modules: %w", err)
		}
		testDirs = append(testDirs, candidates...)
	}

	modules := []ModuleTest{}

	fmt.Println()
	fmt.Println("Found the following directories to test:")
	for _, d := range testDirs {
		rd, err := filepath.Rel(wd, d)
		if err != nil {
			rd = d
		}
		rd = filepath.ToSlash(rd)
		fmt.Printf("\t- %s\n", rd)

		modules = append(modules, ModuleTest{
			Name:    rd,
			DirPath: d,
		})

		// TODO: check for step var files
	}
	fmt.Println()
	fmt.Println("Running tests...")
	fmt.Println()

	if len(tfbins) != 1 {
		return fmt.Errorf("no tf binary located")
	}

	fail := false

	// TODO: handle matrix of tfbins, etc
	for _, m := range modules {
		tf, err := tfexec.NewTerraform(m.DirPath, tfbins[0])
		if err != nil {
			return fmt.Errorf("unable to create Terraform executor: %w", err)
		}

		// tfv, _, err := tf.Version(ctx, false)
		// if err != nil {
		// 	return fmt.Errorf("unable to determine Terraform version: %w", err)
		// }

		fmt.Printf("=== RUN   %s\n", m.Name)

		err = m.Test(ctx, tf)
		if err != nil {
			return fmt.Errorf("unable to test module: %q: %w", m.Name, err)
		}

		// TODO: if versbose or failing
		if m.Failed {
			scanner := bufio.NewScanner(m.output)
			for scanner.Scan() {
				fmt.Print("\t")
				fmt.Println(scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("unable to output results of module %q test: %w", m.Name, err)
			}
		}

		if m.Failed {
			fail = true
			fmt.Printf("--- FAIL: %s\n", m.Name)
		} else {
			fmt.Printf("--- PASS: %s\n", m.Name)
		}
	}

	if fail {
		fmt.Println("FAIL")
		os.Exit(2)
	} else {
		fmt.Println("PASS")
	}

	return nil
}

var (
	skipDirs = []string{
		// the local terraform cache
		".terraform",

		// conventional testing directory
		"testdata",

		// module definition directory, these are not root modules
		// for testing based on the standard definition
		//
		// TODO: make this configurable?
		"modules",
	}
)

func findRootModules(startDir string) ([]string, error) {
	dirsWithTFFiles := map[string]bool{}
	err := filepath.Walk(startDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			for _, s := range skipDirs {
				if s == base {
					return filepath.SkipDir
				}
			}

			// only looking for files
			return nil
		}
		if filepath.Ext(path) == ".tf" {
			dir := filepath.Dir(path)
			dirsWithTFFiles[dir] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to walk: %w", err)
	}

	var rootModules []string
	for d := range dirsWithTFFiles {
		rootModules = append(rootModules, d)
	}
	sort.Strings(rootModules)

	return rootModules, nil
}
