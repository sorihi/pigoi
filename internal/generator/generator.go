package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sorihi/pigoi/templates"
)

type Options struct {
	Name     string
	Module   string
	Template string
}

func Generate(opts Options) error {
	if opts.Name == "" || opts.Name == "." ||
		opts.Name == ".." ||
		filepath.Base(opts.Name) != opts.Name {
		return fmt.Errorf("invalid project name: %q", opts.Name)
	}

	if opts.Template == "" ||
		strings.ContainsAny(opts.Template, `/\`) ||
		opts.Template == "." || opts.Template == ".." {
		return fmt.Errorf("invalid template name")
	}

	root := opts.Template

	if _, err := fs.Stat(templates.FS, root); err != nil {
		return fmt.Errorf("template %q not found", opts.Template)
	}

	target := opts.Name

	// Avoid overwriting existing projects.
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("directory %q already exists", target)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Mkdir(target, 0755); err != nil {
		return err
	}

	// Remove partially generated output on failure.
	completed := false
	defer func() {
		if !completed {
			_ = os.RemoveAll(target)
		}
	}()

	if err := render(templates.FS, root, target, opts); err != nil {
		return err
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = target
	tidy.Env = append(tidy.Environ(), "GOWORK=off")
	if output, err := tidy.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy in %q: %w\n%s", target, err, output)
	}

	completed = true
	return nil
}

func render(source fs.FS, root, target string, opts Options) error {
	return fs.WalkDir(source, root,
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			destination := filepath.Join(target, relative)

			if entry.IsDir() {
				return os.MkdirAll(destination, 0755)
			}

			content, err := fs.ReadFile(source, path)
			if err != nil {
				return err
			}

			// Render files with .tmpl extension.
			if strings.HasSuffix(destination, ".tmpl") {
				destination = strings.TrimSuffix(
					destination, ".tmpl",
				)

				tmpl, err := template.New(path).
					Option("missingkey=error").
					Parse(string(content))
				if err != nil {
					return err
				}

				var buffer bytes.Buffer
				if err := tmpl.Execute(&buffer, opts); err != nil {
					return err
				}

				content = buffer.Bytes()
			}

			return os.WriteFile(destination, content, 0644)
		})

}
