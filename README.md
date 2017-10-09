mstat -- measure memory usage of a program over time
====================================================

This tool only runs on Linux, and requires being installed installed
set-UID.  Build it the normal way:

    $ git clone https://github.com/bpowers/mstat
    $ cd mstat
    $ make; sudo make install

Then, use it to measure memory usage over time:

    $ mstat -o data/mem.tsv -freq 59 -- ./test

And there is even a handy flag to modify the environment:

    $ mstat -o data/mem.tsv -freq 59 -env LD_PRELOAD=libawesome.so -- ./test

It uses the Linux kernel `cgroups` API to create a new memory
controller, and run the program under test in that.

TODO
----

The [Memory
API](https://godoc.org/github.com/containerd/cgroups#MemoryStat) we
are using gives us not only detailed information about the programs
memory usage, but _also_ about kernel memory allocated on behalf of
the program.  We should surface that.
