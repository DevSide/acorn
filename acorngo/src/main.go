package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
)

// import {readFileSync as readFile} from "fs"
// import * as acorn from "acorn"

type BinOptions struct {
	inputFilePaths []string
	forceFileName  bool
	fileMode       bool
	silent         bool
	compact        bool
	tokenize       bool
}

var binOpts = BinOptions{}
var opts = RawOptions{}

func help(status int) {
	std := os.Stdin
	if status == 0 {
		std = os.Stderr
	}
	fmt.Fprintln(std, "usage: "+path.Base(flag.Args()[1])+" [--ecma3|--ecma5|--ecma6|--ecma7|--ecma8|--ecma9|...|--ecma2015|--ecma2016|--ecma2017|--ecma2018|...]")
	fmt.Fprintln(std, "        [--tokenize] [--locations] [--allow-hash-bang] [--allow-await-outside-function] [--compact] [--silent] [--module] [--help] [--] [<infile>...]")
	os.Exit(status)
}

func createOptionsFromArgs(args []string) RawOptions {
	options := RawOptions{}
	for i := 2; i < len(args); i++ {
		arg := args[i]
		if arg[0] != '-' || arg == "-" {
			append(binOpts.inputFilePaths, arg)
		} else if arg == "--" {
			append(binOpts.inputFilePaths, args[0:i+1])
			binOpts.forceFileName = true
			break
		} else if arg == "--locations" {
			options.locations = true
		} else if arg == "--allow-hash-bang" {
			options.allowHashBang = true
		} else if arg == "--allow-await-outside-function" {
			options.allowAwaitOutsideFunction = true
		} else if arg == "--silent" {
			binOpts.silent = true
		} else if arg == "--compact" {
			binOpts.compact = true
		} else if arg == "--help" {
			help(0)
		} else if arg == "--tokenize" {
			binOpts.tokenize = true
		} else if arg == "--module" {
			options.sourceType = "module"
		} else {
			r, _ := regexp.Compile("^--ecma(\\d+)$")
			match := r.FindStringSubmatch(arg)
			if len(match) > 1 {
				options.ecmaVersion = match[1]
			} else {
				options.ecmaVersion = "null"
				help(1)
			}
		}
	}

	return options
}

func run(codeList [][]byte, opts RawOptions) {
	result := []string{}
	fileIdx := 0

	error := func(errMessage string) {
		if !binOpts.fileMode {
			fmt.Fprintln(os.Stderr, errMessage)
		} else {
			// TODO: error for multiple files
			// console.error(binOpts.fileMode ? errMessage.replace(/\(\d+:\d+\)$/, m => m.slice(0, 1) + inputFilePaths[fileIdx] + " " + m.slice(1)) : errMessage)
			fmt.Fprintln(os.Stderr, errMessage)
		}

		os.Exit(1)
	}

	for idx, code := range codeList {
		fileIdx = idx
		if !binOpts.tokenize {
			result, err = NewParser(opts, code, -1).parse()
			if err {
				error(err)
			}
			opts.program = result
		} else {
			tokenizer := acorn.Tokenizer(code, opts), token

			for {
				token := tokenizer.GetToken()
				append(result, token)

				if token._type != acorn.tokTypes.eof {
					break
				}
			}
		}
	}

	if !binOpts.silent {
		// TODO: format indentation
		// console.log(JSON.stringify(result, null, compact ? null : 2))
		fmt.Println(result)
	}
}

func main() {
	options := createOptionsFromArgs(flag.Args())

	if len(binOpts.inputFilePaths) > 0 && (binOpts.forceFileName || !includes(binOpts.inputFilePaths, "-") || len(binOpts.inputFilePaths) != 1) {
		inputFileBytes := [][]byte{}

		for i, path := range binOpts.inputFilePaths {
			data, _ := os.ReadFile(path)
			inputFileBytes[i] = data
		}

		run(inputFileBytes, options)
	} else {
		// TODO: pipe data
		// let code = ""
		// process.stdin.resume()
		// process.stdin.on("data", chunk => code += chunk)
		// process.stdin.on("end", () => run([code]))
	}
}
