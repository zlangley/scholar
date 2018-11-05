// Copyright © 2018 Eiji Onchi <eiji@onchi.me>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/cgxeiji/scholar/scholar"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

func edit(entry *scholar.Entry) {
	key := entry.GetKey()
	saveTo := filepath.Join(libraryPath(), key)

	file := filepath.Join(saveTo, "entry.yaml")

	err := editor(file)
	if err != nil {
		panic(err)
	}

	d, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}

	yaml.Unmarshal(d, &entry)
}

func update(entry *scholar.Entry) {
	key := entry.GetKey()
	saveTo := filepath.Join(libraryPath(), key)

	file := filepath.Join(saveTo, "entry.yaml")

	d, err := yaml.Marshal(entry)
	if err != nil {
		panic(err)
	}

	ioutil.WriteFile(file, d, 0644)
}

func editor(file string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	default:
		cmd = viper.GetString("GENERAL.editor")
	}
	args = append(args, file)
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c.Run()
}

func open(file string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, file)

	return exec.Command(cmd, args...).Start()
}

func clean(filename string) string {
	rx, err := regexp.Compile("[^[:alnum:][:space:]]+")
	if err != nil {
		return filename
	}

	filename = rx.ReplaceAllString(filename, " ")
	filename = strings.Replace(filename, " ", "_", -1)

	return strings.ToLower(filename)
}

func libraryPath() string {
	if currentLibrary != "" {
		if !viper.Sub("LIBRARIES").IsSet(currentLibrary) {
			fmt.Println("No library called", currentLibrary, "was found!")
			fmt.Println("Available libraries:")
			for k, v := range viper.GetStringMapString("LIBRARIES") {
				fmt.Println(" ", k)
				fmt.Println("   ", v)
			}
			os.Exit(1)
		}

		return viper.Sub("LIBRARIES").GetString(currentLibrary)
	}
	return viper.Sub("LIBRARIES").GetString(viper.GetString("GENERAL.default"))
}

func entryList() []*scholar.Entry {
	path := libraryPath()
	dirs, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println(err)
		fmt.Println(`
Add an entry to create this directory or run:

	scholar config

to set the correct path of this library.
`)
		os.Exit(1)
	}

	var wg sync.WaitGroup

	entries := []*scholar.Entry{}
	queue := make(chan *scholar.Entry)
	done := make(chan bool)

	go func() {
		defer close(done)
		for e := range queue {
			entries = append(entries, e)
		}
	}()

	wg.Add(len(dirs))
	for _, dir := range dirs {
		dir := dir
		go func() {
			defer wg.Done()
			if dir.IsDir() {
				d, err := ioutil.ReadFile(filepath.Join(path, dir.Name(), "entry.yaml"))
				if err != nil {
					panic(err)
				}
				var e scholar.Entry
				if err := yaml.Unmarshal(d, &e); err != nil {
					panic(err)
				}

				checkDirKey(path, dir.Name(), &e)
				queue <- &e
			}
		}()
	}
	wg.Wait()
	close(queue)
	<-done

	return entries
}

// checkDirKey makes sure the directory name is the same as the entry's key.
func checkDirKey(path, dir string, e *scholar.Entry) {
	if dir == e.GetKey() {
		return
	}
	if err := os.Rename(filepath.Join(path, dir), filepath.Join(path, ".tmp.scholar")); err != nil {
		panic(err)
	}

	e.Key = getUniqueKey(e.GetKey())

	if err := os.Rename(filepath.Join(path, ".tmp.scholar"), filepath.Join(path, e.GetKey())); err != nil {
		panic(err)
	}
	update(e)
	fmt.Println("Renamed:")
	fmt.Println(" ", filepath.Join(path, dir), ">",
		filepath.Join(path, e.GetKey()))
}