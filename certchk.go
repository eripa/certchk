/*
certchk - check certificates of https sites

Copyright (c) 2016 RapidLoop

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

const toolVersion = "0.2"

var (
	dialer       = &net.Dialer{Timeout: 5 * time.Second}
	domainFile   string
	scriptOutput bool
	versionCheck bool
)

func init() {
	flag.BoolVar(&scriptOutput, "s", false, "produce less verbose output")
	flag.StringVar(&domainFile, "f", "", "read server names from `file`")
	flag.BoolVar(&versionCheck, "version", false, "show tool version")
}

func check(server string, width int) {
	conn, err := tls.DialWithDialer(dialer, "tcp", server+":443", nil)
	if err != nil {
		if scriptOutput {
			fmt.Printf("%*s error 1970-01-01 (%v)\n", width, server, err)
		} else {
			fmt.Printf("%*s | %v\n", width, server, err)
		}
		return
	}
	defer conn.Close()
	valid := conn.VerifyHostname(server)

	for _, c := range conn.ConnectionState().PeerCertificates {
		if valid == nil {
			if scriptOutput {
				fmt.Printf("%*s valid %s\n", width, server,
					c.NotAfter.Format("2006-01-02"))
			} else {
				fmt.Printf("%*s | valid, expires on %s (%s)\n", width, server,
					c.NotAfter.Format("2006-01-02"), humanize.Time(c.NotAfter))
			}
		} else {
			fmt.Printf("%*s, %v\n", width, server, valid)
		}
		return
	}
}

func main() {
	// parse command-line args
	flag.Parse()
	if versionCheck {
		fmt.Printf("%s v%s\n", os.Args[0], toolVersion)
		os.Exit(0)
	}
	if flag.NArg() == 0 && len(domainFile) == 0 {
		fmt.Fprintf(os.Stderr, "Usage of certchk:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// collect list of server names
	names := getNames()

	// for cosmetics
	width := 0
	for _, name := range names {
		if len(name) > width {
			width = len(name)
		}
	}

	if !scriptOutput {
		fmt.Printf("%*s | Certificate status\n%s-+-%s\n", width, "Server",
			strings.Repeat("-", width), strings.Repeat("-", 80-width-2))
	}
	// actually check
	// channel for synchronizing 'done state', buffer the amount of names
	done := make(chan bool, len(names))
	for _, name := range names {
		go func(name string) {
			check(name, width)
			done <- true
		}(name)
	}

	// Drain the channel and wait for all goroutines to complete
	for i := 0; i < len(names); i++ {
		<-done // wait for one task to complete
	}
}

func getNames() (names []string) {

	// read names from the file
	if len(domainFile) > 0 {
		f, err := os.Open(domainFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if len(line) > 0 && line[0] != '#' {
				names = append(names, strings.Fields(line)[0])
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)
			os.Exit(1)
		}
		f.Close()
	}

	// add names specified on the command line
	names = append(names, flag.Args()...)
	return
}
