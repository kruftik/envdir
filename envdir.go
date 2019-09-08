package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"
)

const (
	ExitCodeOk    = 0
	ExitCodeFatal = 111
)

type Envdir struct {
	outStream, errStream io.Writer
	env                  []string
}

func (e *Envdir) log(msg string, w io.Writer) {
	// 2019-09-08 10:47:42.433 MSK [18] LOG
	t := time.Now()
	tz, _ := t.Zone()

	msg = fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d.000 %s [X] ENVDIR:\t%s", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), tz, msg)

	if w != nil {
		fmt.Fprint(w, msg)
	} else {
		fmt.Print(msg)
	}
}

func (e *Envdir) fatal(msg string) int {
	e.log(msg, e.errStream)
	return ExitCodeFatal
}

func (e *Envdir) run(args []string) int {
	if len(args) < 3 {
		return e.fatal("usage: envdir dir command\n")
	}

	dir := args[1]
	child := args[2]
	childArgs := args[3:]

	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return e.fatal(fmt.Sprintf("%s\n", err.Error()))
	}

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			return e.fatal(fmt.Sprintf("%s is not a file, but a directory\n", fileInfo.Name()))
		}

		fileName := fileInfo.Name()
		if strings.HasPrefix(fileName, ".") {
			continue
		}

		filePath := path.Join(dir, fileName)
		file, err := os.Open(filePath)
		if err != nil {
			return e.fatal(fmt.Sprintf("%s\n", err.Error()))
		}

		fsize := fileInfo.Size()
		if fsize == 0 {
			for i, elem := range e.env {
				if strings.HasPrefix(elem, fileName+"=") {
					// remove env
					e.env = append(e.env[:i], e.env[i+1:]...)
					break
				}
			}
			continue
		}

		data := make([]byte, fsize)
		n, err := file.Read(data)
		if err != nil {
			return e.fatal(fmt.Sprintf("%s\n", err.Error()))
		}
		if int64(n) != fsize {
			return e.fatal(fmt.Sprintf("invalid file read size, got: %s, expected: %s, \n", n, fsize))
		}

		v := strings.SplitN(string(data), "\n", 2)[0]
		v = strings.Replace(v, "\x00", "\n", -1) // replace NULL character with newline
		v = strings.TrimRight(v, " \t")          // trim trailing space and tab

		e.env = append(e.env, fileName+"="+v)
	}

	binary, err := exec.LookPath(child)
	if err != nil {
		return e.fatal(fmt.Sprintf("Cannot find '%s': %s\n", child, err))
	}

	err = syscall.Exec(binary, append([]string{child}, childArgs...), e.env)
	if err != nil {
		return e.fatal(fmt.Sprintf("Cannot start '%s': %s\n", child, err))
	}

	return ExitCodeOk
}
