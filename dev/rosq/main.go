// rosq is a tiny RouterOS API client for poking a device during development.
// Usage: rosq [-a 127.0.0.1:8728] [-u admin] [-p ”] /cmd/path [=attr=val|?query ...] [-- /next/cmd ...]
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-routeros/routeros/v3"
)

func main() {
	addr := flag.String("a", "127.0.0.1:8728", "address")
	user := flag.String("u", "admin", "user")
	pass := flag.String("p", "", "password")
	to := flag.Duration("t", 10*time.Second, "timeout")
	quiet := flag.Bool("q", false, "only exit status (login probe)")
	flag.Parse()
	c, err := routeros.DialTimeout(*addr, *user, *pass, *to)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial/login:", err)
		os.Exit(2)
	}
	defer c.Close()
	if *quiet {
		return
	}
	var cmds [][]string
	cur := []string{}
	for _, a := range flag.Args() {
		if a == "--" {
			if len(cur) > 0 {
				cmds = append(cmds, cur)
			}
			cur = []string{}
			continue
		}
		cur = append(cur, a)
	}
	if len(cur) > 0 {
		cmds = append(cmds, cur)
	}
	rc := 0
	for _, cmd := range cmds {
		fmt.Printf(">>> %s\n", strings.Join(cmd, " "))
		r, err := c.RunArgs(cmd)
		if err != nil {
			fmt.Printf("!!! error: %v\n", err)
			rc = 1
			continue
		}
		for i, s := range r.Re {
			fmt.Printf("!re #%d\n", i)
			for _, p := range s.List {
				fmt.Printf("  %s=%q\n", p.Key, p.Value)
			}
		}
		if r.Done != nil && len(r.Done.List) > 0 {
			fmt.Printf("!done")
			for _, p := range r.Done.List {
				fmt.Printf(" %s=%q", p.Key, p.Value)
			}
			fmt.Println()
		} else {
			fmt.Printf("!done (%d replies)\n", len(r.Re))
		}
	}
	os.Exit(rc)
}
