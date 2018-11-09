// Copyright 2017 Bobby Powers. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	outputPath = flag.String("o", "", "file to store data as a TSV")
	verbose    = flag.Bool("v", false, "verbose logging")
	frequency  = flag.Int("freq", 10, "frequency in Hz for memory sampling")
)

type envFlags []string

func (ef *envFlags) String() string {
	return strings.Join(*ef, ",")
}

func (ef *envFlags) Set(value string) error {
	*ef = append(*ef, value)
	return nil
}

func (ef *envFlags) Get() interface{} {
	return *ef
}

var envVars envFlags

func newPath() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("rand.Read: %s", err))
	}

	encoded := hex.EncodeToString(buf)

	return fmt.Sprintf("/mstat-%s", encoded)
}

func execInNamespace(fd int, args []string) int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	gid := syscall.Getgid()
	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		log.Fatalf("Setresgid(%d): %s", gid, err)
	}

	uid := syscall.Getuid()
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		log.Fatalf("Setresuid(%d): %s", uid, err)
	}

	rpipe := os.NewFile(uintptr(fd), "rpipe")

	env := os.Environ()
	for _, envVar := range envVars {
		env = append(env, envVar)
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
	flag.Var(&envVars, "env", "add an environmental variable")
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
	childArgs := []string{}
	for _, envVar := range envVars {
		childArgs = append(childArgs, "-env", envVar)
	}
	childArgs = append(childArgs, "--", internalFlag)
	childArgs = append(childArgs, args...)
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

	// give the OS a chance to exec our child and have it waiting
	// at read(pipe)
	time.Sleep(10 * time.Millisecond)

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

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	gid := syscall.Getgid()
	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		log.Fatalf("Setresgid(%d): %s", gid, err)
	}

	uid := syscall.Getuid()
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		log.Fatalf("Setresuid(%d): %s", uid, err)
	}

	stats := poller.End()
	// - (check cgroup is empty?)
	// - report on total memory usage

	var buf bytes.Buffer
	bio := bufio.NewWriter(&buf)
	if _, err := bio.WriteString("time\trss\n"); err != nil {
		log.Fatalf("bio.WriteString: %s", err)
	}

	start := stats.Rss[0].Time.UnixNano()
	for i := 0; i < len(stats.Rss); i++ {
		r := stats.Rss[i]
		line := fmt.Sprintf("%d\t%d\n", r.Time.UnixNano()-start, r.Value)
		if _, err := bio.WriteString(line); err != nil {
			log.Fatalf("bufio.WriteString: %s", err)
		}
	}
	if err = bio.Flush(); err != nil {
		log.Fatalf("bio.Flush: %s", err)
	}

	if *outputPath != "" {
		if err = ioutil.WriteFile(*outputPath, buf.Bytes(), 0666); err != nil {
			log.Fatalf("writing output to '%s' failed: %s", *outputPath, err)
		}
	}

	if *verbose && len(stats.Stats) > 10 {
		buf, err := json.Marshal(stats.Stats[len(stats.Stats)-10])
		if err != nil {
			log.Fatalf("json.Marshal: %s", err)
		}

		var out bytes.Buffer
		json.Indent(&out, buf, "", "    ")
		out.WriteTo(os.Stdout)

		// start := stats.Rss[0].Time.UnixNano()
		// for i := 0; i < len(stats.Rss); i++ {
		// 	r := stats.Rss[i]
		// 	fmt.Printf("\t%d\t%d\n", r.Time.UnixNano()-start, r.Value)
		// }
	}
}
