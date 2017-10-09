// Copyright 2017 Bobby Powers. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	usage = `Usage: %s [OPTION...] [--] PROG
Run a command, reporting on its memory usage over time.

Options:
`
)

var (
	verbose   = flag.Bool("v", false, "verbose logging")
	frequency = flag.Int("freq", 10, "frequency for memory sampling")
	extraEnv  = flag.String("env", "", "environment variable to set in the child")
)

func newPath() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("rand.Read: %s", err))
	}

	encoded := hex.EncodeToString(buf)

	return fmt.Sprintf("/mstat-%s", encoded)
}

func execInNamespace(fd int, args []string) int {
	rpipe := os.NewFile(uintptr(fd), "rpipe")

	*extraEnv = strings.TrimSpace(*extraEnv)
	env := os.Environ()
	if *extraEnv != "" {
		env = append(env, *extraEnv)
	}

	buf := make([]byte, 16)

	// block until our parent mstat process sends us the 'all
	// clear to exec' signal, which is writing the string "ok" to
	// the pipe.
	if n, err := rpipe.Read(buf); n != 2 || err != nil || string(buf[:2]) != "ok" {
		log.Fatalf("internal error: Read: %s/%d/%#v", err, n, buf)
	}

	rpipe.Close()
	rpipe = nil

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return 1
	} else {
		return 0
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if strings.HasPrefix(args[0], "--INTERNAL_FD=") {
		fd, err := strconv.Atoi(args[0][len("--INTERNAL_FD="):])
		if err != nil {
			log.Fatalf("internal error: Atoi('%s'): %s", args[0], err)
		}
		if fd < 0 {
			log.Fatalf("internal error: expected unsigned int, not %d", fd)
		}
		args = args[1:]
		if len(args) < 1 {
			log.Fatalf("internal error: no args")
		}

		os.Exit(execInNamespace(fd, args))
	}

	cgroupPath := cgroups.StaticPath(newPath())

	memLimit := int64(4 * 1024 * 1024 * 1024) // 4 GB
	cgroup, err := cgroups.New(cgroups.V1, cgroupPath, &specs.LinuxResources{
		//CPU: &specs.LinuxCPU{},
		Memory: &specs.LinuxMemory{
			Limit: &memLimit,
		},
	})
	if err != nil {
		log.Fatalf("cgroups.New: %s", err)
	}
	defer cgroup.Delete()

	// create a pipe to signal to our child when it should
	// actually start running (after we stick it into the cgroup)
	r, w, err := os.Pipe()
	if err != nil {
		log.Fatalf("os.Pipe: %s", err)
	}

	// use 3 as the file descriptor, as that refers to the first
	// FD when using ExtraFiles
	internalFlag := fmt.Sprintf("--INTERNAL_FD=%d", 3) // int(r.Fd())
	childArgs := append([]string{"-env", *extraEnv, "--", internalFlag}, args...)
	cmd := exec.Command(os.Args[0], childArgs...)
	cmd.ExtraFiles = []*os.File{r}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Start(); err != nil {
		log.Fatalf("Start: %s", err)
	}

	r.Close()
	r = nil

	if err := cgroup.Add(cgroups.Process{Pid: cmd.Process.Pid}); err != nil {
		log.Fatalf("cg.Add: %s", err)
	}

	poller, err := NewPoller(cgroup, *frequency)
	if err != nil {
		log.Fatalf("NewPoller: %s", err)
	}

	if n, err := w.Write([]byte("ok")); n != 2 || err != nil {
		log.Fatalf("pipe.Write: %d/%s", n, err)
	}
	w.Close()
	w = nil

	err = cmd.Wait()
	if err != nil {
		exitError := err.(*exec.ExitError)
		log.Printf("error: %#v", exitError)
		os.Exit(1)
	}

	stats := poller.End()
	// - (check cgroup is empty?)
	// - report on total memory usage

	if *verbose {
		for i := 0; i < len(stats.Rss); i++ {
			r := stats.Rss[i]
			fmt.Printf("\t%d\t%d\n", r.Time.UnixNano(), r.Value)
		}
	}
}
