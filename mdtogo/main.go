// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

// Package main generates cobra.Command go variables containing documentation read from .md files.
// Usage: mdtogo SOURCE_MD_DIR/ DEST_GO_DIR/ [--recursive=true] [--license=license.txt|none]
//
// The command will create a docs.go file under DEST_GO_DIR/ containing string variables to be
// used by cobra commands for documentation. The variable names are generated from the name of
// the directory in which the files resides, replacing '-' with '', title casing the name.
// All *.md files will be read from DEST_GO_DIR/, including subdirectories if --recursive=true,
// and a single DEST_GO_DIR/docs.go file is generated.
//
// The content for each of the three variables created per folder, are set
// by looking for a HTML comment on one of two forms:
//
// <!--mdtogo:<VARIABLE_NAME>-->
//   ..some content..
// <!--mdtogo-->
//
// or
//
// <!--mdtogo:<VARIABLE_NAME>
// ..some content..
// -->
//
// The first are for content that should show up in the rendered HTML, while
// the second is for content that should be hidden in the rendered HTML.
//
// <VARIABLE_NAME> must be one of Short, Long or Examples.
//
// Flags:
//   --recursive=true
//     Scan the directory structure recursively for .md files
//   --license
//     Controls the license header added to the files.  Specify a path to a license file,
//     or "none" to skip adding a license.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var recursive bool
var licenseFile string

func main() {
	for _, a := range os.Args {
		if a == "--recursive=true" {
			recursive = true
		}
		if strings.HasPrefix(a, "--license=") {
			licenseFile = strings.ReplaceAll(a, "--license=", "")
		}
	}

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: mdtogo SOURCE_MD_DIR/ DEST_GO_DIR/\n")
		os.Exit(1)
	}
	source := os.Args[1]
	dest := os.Args[2]

	files, err := readFiles(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	var docs []doc
	for _, path := range files {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		parsedDoc := parse(path, string(b))

		docs = append(docs, parsedDoc)
	}

	var license string

	switch licenseFile {
	case "":
		license = `// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0`
	case "none":
		// no license -- maybe added by another tool
	default:
		b, err := ioutil.ReadFile(licenseFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		license = string(b)
	}

	out := []string{license, `
// Code generated by "mdtogo"; DO NOT EDIT.
package ` + filepath.Base(dest) + "\n"}

	for i := range docs {
		out = append(out, docs[i].String())
	}

	if _, err := os.Stat(dest); err != nil {
		_ = os.Mkdir(dest, 0700)
	}

	o := strings.Join(out, "\n")
	err = ioutil.WriteFile(filepath.Join(dest, "docs.go"), []byte(o), 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func readFiles(source string) ([]string, error) {
	filePaths := make([]string, 0)
	if recursive {
		err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if filepath.Ext(info.Name()) == ".md" {
				filePaths = append(filePaths, path)
			}
			return nil
		})
		if err != nil {
			return filePaths, err
		}
	} else {
		files, err := ioutil.ReadDir(source)
		if err != nil {
			return filePaths, err
		}
		for _, info := range files {
			if filepath.Ext(info.Name()) == ".md" {
				path := filepath.Join(source, info.Name())
				filePaths = append(filePaths, path)
			}
		}
	}
	return filePaths, nil
}

var (
	mdtogoTag         = regexp.MustCompile(`<!--mdtogo:(Short|Long|Examples)-->([\s\S]*?)<!--mdtogo-->`)
	mdtogoInternalTag = regexp.MustCompile(`<!--mdtogo:(Short|Long|Examples)\s+?([\s\S]*?)-->`)
)

func parse(path, value string) doc {
	pathDir := filepath.Dir(path)
	_, name := filepath.Split(pathDir)

	name = strings.Title(name)
	name = strings.ReplaceAll(name, "-", "")

	matches := mdtogoTag.FindAllStringSubmatch(value, -1)
	matches = append(matches, mdtogoInternalTag.FindAllStringSubmatch(value, -1)...)

	var doc doc
	for _, match := range matches {
		switch match[1] {
		case "Short":
			val := strings.TrimSpace(match[2])
			doc.Short = val
		case "Long":
			val := cleanUpContent(match[2])
			doc.Long = val
		case "Examples":
			val := cleanUpContent(match[2])
			doc.Examples = val
		}
	}
	doc.Name = name
	return doc
}

func cleanUpContent(text string) string {
	var lines []string

	scanner := bufio.NewScanner(bytes.NewBufferString(strings.Trim(text, "\n")))

	indent := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "```") {
			indent = !indent
			continue
		}

		if indent {
			line = "  " + line
		}

		line = strings.ReplaceAll(line, "`", "` + \"`\" + `")

		lines = append(lines, line)
	}

	return fmt.Sprintf("\n%s\n", strings.Join(lines, "\n"))
}

type doc struct {
	Name     string
	Short    string
	Long     string
	Examples string
}

func (d doc) String() string {
	var parts []string

	if d.Short != "" {
		parts = append(parts,
			fmt.Sprintf("var %sShort = `%s`", d.Name, d.Short))
	}
	if d.Long != "" {
		parts = append(parts,
			fmt.Sprintf("var %sLong = `%s`", d.Name, d.Long))
	}
	if d.Examples != "" {
		parts = append(parts,
			fmt.Sprintf("var %sExamples = `%s`", d.Name, d.Examples))
	}

	return strings.Join(parts, "\n") + "\n"
}
