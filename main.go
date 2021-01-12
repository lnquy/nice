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

	"github.com/fatih/color"
	"github.com/tidwall/gjson"
)

var (
	fInputFiles   string
	fOutputFormat string
	fFieldColors  string
)

func init() {
	flag.StringVar(&fInputFiles, "files", "", "List of path input log files, separated by comma (,)")
	flag.StringVar(&fOutputFormat, "f", "", "Output format. Fields can be access by dot notation path, separated by comma (,)")
	flag.StringVar(&fFieldColors, "colors", "", "Field colors")
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
	outColors := getColorFormat(fFieldColors)

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
			pipeStdin(outFields, outColors, outputWriter)
		}()
	}

	wg := sync.WaitGroup{}
	ctx, ctxCancel := context.WithCancel(context.Background())
	for _, inFile := range fileStrs {
		wg.Add(1)
		go pipeFile(ctx, &wg, inFile, outFields, outColors, outputWriter)
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

func pipeStdin(outputFields []string, outColors []*color.Color, out io.Writer) {
	reader := bufio.NewReader(os.Stdin)

	buff := bytes.NewBuffer(make([]byte, 0, 1024))
	for {
		line, _, err := reader.ReadLine()
		if err != nil && err == io.EOF {
			return
		}

		// Grep JSON
		buff.Reset()
		print(line, outputFields, outColors, buff, out)
	}
}

func pipeFile(ctx context.Context, wg *sync.WaitGroup, filepath string, outputFields []string, outColors []*color.Color, out io.Writer) {
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
			print(scanner.Bytes(), outputFields, outColors, buff, out)
		}
	}
}

func print(line []byte, outFields []string, outColors []*color.Color, buff *bytes.Buffer, out io.Writer) {
	jsonLine := gjson.ParseBytes(line)

	for idx, field := range outFields {
		jsField := jsonLine.Get(field)
		val := jsField.Str
		if jsField.Type == gjson.JSON {
			val = jsField.Raw
		}
		if strings.TrimSpace(val) == "" {
			continue
		}

		if idx < len(outColors) { // Has color format
			buff.WriteString(outColors[idx].Sprintf("%s\t", val))
		} else {
			buff.WriteString(val + "\t")
		}
	}

	if buff.Len() == 0 {
		return
	}
	buff.WriteString("\n")
	if _, err := fmt.Fprintf(out, "%s", buff.Bytes()); err != nil {
		log.Printf("nice: failed to write to output: %s. Log: %s", err, buff.Bytes())
	}
}

func getColorFormat(inStr string) []*color.Color {
	if len(inStr) == 0 || strings.TrimSpace(inStr) == "" {
		return nil
	}

	colors := strings.Split(inStr, ",")
	var outColors []*color.Color
	for _, c := range colors {
		switch strings.ToLower(strings.TrimSpace(c)) {
		case "black":
			outColors = append(outColors, color.New(color.FgBlack))
		case "red":
			outColors = append(outColors, color.New(color.FgRed))
		case "green":
			outColors = append(outColors, color.New(color.FgGreen))
		case "yellow":
			outColors = append(outColors, color.New(color.FgYellow))
		case "blue":
			outColors = append(outColors, color.New(color.FgBlue))
		case "magenta":
			outColors = append(outColors, color.New(color.FgMagenta))
		case "cyan":
			outColors = append(outColors, color.New(color.FgCyan))
		case "white":
			outColors = append(outColors, color.New(color.FgWhite))
		default:
			outColors = append(outColors, color.New(color.Reset))
		}
	}

	return outColors
}
