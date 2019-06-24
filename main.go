package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/tidwall/gjson"
)

var (
	fInputFiles   string
	fOutputFormat string
)

func init() {
	flag.StringVar(&fInputFiles, "files", "", "List of path input log files, separated by comma (,)")
	flag.StringVar(&fOutputFormat, "f", "", "Output format. Fields can be access by dot notation path, separated by comma (,)")
}

func main() {
	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Println(`
Examples:
  $ nice --files 20190624.log -f time,msg
  $ myapp | nice -f time,level,msg
  $ myapp | nice --files 20190624.log,anotherlogfile.log -f time,level,msg,field.child.id`)
	}
	flag.Parse()

	var fileStrs []string
	if fInputFiles != "" {
		fileStrs = strings.Split(fInputFiles, ",")
	}
	outFields := strings.Split(fOutputFormat, ",")

	outputWriter := os.Stdout

	// Read from stdin
	fi, err := os.Stdin.Stat()
	if err != nil {
		log.Panicf("nice: failed to get stdin info")
	}
	// Standalone rune without stdin pipe (|) => Skip reading from stdin
	isPiped := (fi.Mode() & os.ModeCharDevice) == 0
	if isPiped {
		// This goroutine continue running until the app stopped
		go func() {
			log.Printf("nice: start reading from stdin")
			pipeStdin(outFields, outputWriter)
		}()
	}

	wg := sync.WaitGroup{}
	ctx, ctxCancel := context.WithCancel(context.Background())
	for _, inFile := range fileStrs {
		wg.Add(1)
		go pipeFile(ctx, &wg, inFile, outFields, outputWriter)
	}

	// Trap signal if reading from stdin
	if isPiped {
		stopChan := make(chan os.Signal, 1)
		signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-stopChan
		log.Printf("nice: %s signal received. Start exiting", sig)
		ctxCancel() // Notify background processes to stop
	}

	wg.Wait()
	if err := outputWriter.Close(); err != nil {
		log.Panicf("nice: failed to close output writer")
	}
	log.Println("nice: exit")
}

func pipeStdin(outputFields []string, out io.Writer) {
	reader := bufio.NewReader(os.Stdin)

	buff := bytes.NewBuffer(make([]byte, 0, 1024))
	for {
		line, _, err := reader.ReadLine()
		if err != nil && err == io.EOF {
			return
		}

		// Grep JSON
		buff.Reset()
		print(line, outputFields, buff, out)
	}
}

func pipeFile(ctx context.Context, wg *sync.WaitGroup, filepath string, outputFields []string, out io.Writer) {
	defer wg.Done()

	f, err := os.OpenFile(filepath, os.O_RDONLY, 0400)
	if err != nil {
		log.Printf("nice: failed to open file %v: %v", filepath, err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("nice: failed to close file %v: %v", filepath, err)
		}
	}()
	scanner := bufio.NewScanner(f)

	buff := bytes.NewBuffer(make([]byte, 0, 1024))
	for {
		select {
		case <-ctx.Done():
			log.Printf("nice: [%v]: context cancel reveiced. Exit", filepath)
			return
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					log.Printf("nice: [%v]: file scanner error: %v", err)
				} else {
					log.Printf("nice: [%v]: all logs processed (EOF). Exit", filepath)
				}
				return
			}

			buff.Reset()
			print(scanner.Bytes(), outputFields, buff, out)
		}
	}
}

func print(line []byte, outFields []string, buff *bytes.Buffer, out io.Writer) {
	for _, field := range outFields {
		jsField := gjson.GetBytes(line, field)
		val := jsField.Str
		if jsField.Type == gjson.JSON {
			val = jsField.Raw
		}
		if strings.TrimSpace(val) == "" {
			continue
		}
		buff.WriteString(val + "\t")
	}

	if buff.Len() == 0 {
		return
	}
	buff.WriteString("\n")
	_, err := out.Write(buff.Bytes())
	if err != nil {
		log.Printf("nice: failed to write to output: %s. Log: %s", err, buff.Bytes())
	}
}
