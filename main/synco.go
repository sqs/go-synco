package main

import "../synco"
import (
	"flag"
	"fmt"
	"os"
)

var query = flag.String("query", "after:2012/09/12", "Gmail query to limit fetch")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: synco [username] [password]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		usage()
	}

	server := &synco.IMAPServer{ "imap.googlemail.com", 993 }
	acct := &synco.IMAPAccount{ args[0], args[1], server}

	synco.PrintMail(acct, *query)
}
