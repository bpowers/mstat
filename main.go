// Copyright 2017 Bobby Powers. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/containerd/cgroups"
)

const (
	usage = `Usage: %s [OPTION...] [--] PROG
Run a command, reporting on its memory usage over time.

Options:
`
)

func main() {
	verbose := flag.Bool("v", false, "verbose logging")
	env := flag.String("env", "", "environment variable to set in the child")

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

	fmt.Printf("args: %#v\n", args)

	control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath("/test"))
	if err != nil {
		log.Fatalf("cgroups.Load: %s", err)
	}

	fmt.Printf("got controller: %#v (%s/%s)\n", control, verbose, env)
}
