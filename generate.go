// Copyright 2017 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
)

const tpl = `// Code generated by go generate; DO NOT EDIT.

package {{ .Package }} // import "miniflux.app/{{ .ImportPath }}"

var {{ .Map }} = map[string]string{
{{ range $constant, $content := .Files }}` + "\t" + `"{{ $constant }}": ` + "`{{ $content }}`" + `,
{{ end }}}

var {{ .Map }}Checksums = map[string]string{
{{ range $constant, $content := .Checksums }}` + "\t" + `"{{ $constant }}": "{{ $content }}",
{{ end }}}
`

var bundleTpl = template.Must(template.New("").Parse(tpl))

type Bundle struct {
	Package    string
	Map        string
	ImportPath string
	Files      map[string]string
	Checksums  map[string]string
}

func (b *Bundle) Write(filename string) {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	bundleTpl.Execute(f, b)
}

func NewBundle(pkg, mapName, importPath string) *Bundle {
	return &Bundle{
		Package:    pkg,
		Map:        mapName,
		ImportPath: importPath,
		Files:      make(map[string]string),
		Checksums:  make(map[string]string),
	}
}

func readFile(filename string) []byte {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return data
}

func checksum(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func basename(filename string) string {
	return path.Base(filename)
}

func stripExtension(filename string) string {
	filename = strings.TrimSuffix(filename, path.Ext(filename))
	return strings.Replace(filename, " ", "_", -1)
}

func glob(pattern string) []string {
	// There is no Glob function in path package, so we have to use filepath and replace in case of Windows
	files, _ := filepath.Glob(pattern)
	for i := range files {
		if strings.Contains(files[i], "\\") {
			files[i] = strings.Replace(files[i], "\\", "/", -1)
		}
	}
	return files
}

func concat(files []string) string {
	var b strings.Builder
	for _, file := range files {
		b.Write(readFile(file))
	}
	return b.String()
}

func generateJSBundle(bundleFile string, bundleFiles map[string][]string, prefixes, suffixes map[string]string) {
	bundle := NewBundle("static", "Javascripts", "ui/static")
	m := minify.New()
	m.AddFunc("text/javascript", js.Minify)

	for name, srcFiles := range bundleFiles {
		var b strings.Builder

		if prefix, found := prefixes[name]; found {
			b.WriteString(prefix)
		}

		b.WriteString(concat(srcFiles))

		if suffix, found := suffixes[name]; found {
			b.WriteString(suffix)
		}

		minifiedData, err := m.String("text/javascript", b.String())
		if err != nil {
			panic(err)
		}

		bundle.Files[name] = minifiedData
		bundle.Checksums[name] = checksum([]byte(minifiedData))
	}

	bundle.Write(bundleFile)
}

func generateCSSBundle(bundleFile string, themes map[string][]string) {
	bundle := NewBundle("static", "Stylesheets", "ui/static")
	m := minify.New()
	m.AddFunc("text/css", css.Minify)

	for theme, srcFiles := range themes {
		data := concat(srcFiles)
		minifiedData, err := m.String("text/css", data)
		if err != nil {
			panic(err)
		}

		bundle.Files[theme] = minifiedData
		bundle.Checksums[theme] = checksum([]byte(minifiedData))
	}

	bundle.Write(bundleFile)
}

func generateBinaryBundle(bundleFile string, srcFiles []string) {
	bundle := NewBundle("static", "Binaries", "ui/static")

	for _, srcFile := range srcFiles {
		data := readFile(srcFile)
		filename := basename(srcFile)
		encodedData := base64.StdEncoding.EncodeToString(data)

		bundle.Files[filename] = string(encodedData)
		bundle.Checksums[filename] = checksum(data)
	}

	bundle.Write(bundleFile)
}

func generateBundle(bundleFile, pkg, mapName string, srcFiles []string) {
	bundle := NewBundle(pkg, mapName, pkg)

	for _, srcFile := range srcFiles {
		data := readFile(srcFile)
		filename := stripExtension(basename(srcFile))

		bundle.Files[filename] = string(data)
		bundle.Checksums[filename] = checksum(data)
	}

	bundle.Write(bundleFile)
}

func main() {
	generateJSBundle("ui/static/js.go", map[string][]string{
		"app": []string{
			"ui/static/js/dom_helper.js",
			"ui/static/js/touch_handler.js",
			"ui/static/js/keyboard_handler.js",
			"ui/static/js/request_builder.js",
			"ui/static/js/modal_handler.js",
			"ui/static/js/app.js",
			"ui/static/js/bootstrap.js",
		},
		"sw": []string{
			"ui/static/js/sw.js",
		},
	}, map[string]string{
		"app": "(function(){'use strict';",
		"sw":  "'use strict';",
	}, map[string]string{
		"app": "})();",
	})

	generateCSSBundle("ui/static/css.go", map[string][]string{
		"default":   []string{"ui/static/css/common.css"},
		"black":     []string{"ui/static/css/common.css", "ui/static/css/black.css"},
		"sansserif": []string{"ui/static/css/common.css", "ui/static/css/sansserif.css"},
	})

	generateBinaryBundle("ui/static/bin.go", glob("ui/static/bin/*"))

	generateBundle("database/sql.go", "database", "SqlMap", glob("database/sql/*.sql"))
	generateBundle("template/views.go", "template", "templateViewsMap", glob("template/html/*.html"))
	generateBundle("template/common.go", "template", "templateCommonMap", glob("template/html/common/*.html"))
	generateBundle("locale/translations.go", "locale", "translations", glob("locale/translations/*.json"))
}
