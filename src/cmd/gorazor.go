package main

// considering import fsnotify

import (
	"flag"
	"fmt"
	"os"

	"gorazor"
)

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage: Specify template file or directory\n")
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {

	var indir, outdir, infile, outfile string
	debug := false
	flag.StringVar(&indir, "indir", "", "Template directory path")
	flag.StringVar(&outdir, "outdir", "", "Output directory path")
	flag.StringVar(&infile, "f", "", "Template file path")
	flag.StringVar(&outfile, "o", "", "Output file path")
	flag.BoolVar(&debug, "d", false, "Enable debug mode")
	flag.Usage = Usage

	flag.Parse()

	options := gorazor.Option{}
	if debug {
		options["Debug"] = true
	}

	if indir != "" && outdir != "" {
		err := gorazor.GenFolder(indir, outdir)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	} else if infile != "" && outfile != "" {
		fmt.Printf("processing: %s %s\n", infile, outfile)
		err := gorazor.GenFile(infile, outfile, options)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	} else {
		flag.Usage()
	}
}
