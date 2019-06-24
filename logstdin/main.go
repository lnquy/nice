package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	f, err := os.OpenFile("20190624.log", os.O_RDONLY, 0400)
	if err != nil {
		log.Panicf("failed to open log file")
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Panicf("scanner error: %v", err)
	}

	ticker := time.NewTicker(time.Second)
	for {
		t := <-ticker.C
		fmt.Printf(`{"level":"debug","msg":"Ticker ticked","time":"%s"}`, t.Format(time.RFC3339Nano))
		fmt.Println()
	}
}
