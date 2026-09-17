package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

func main() {
	raw, err := os.ReadFile("bin\\smoke-report.pdf")
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	re := regexp.MustCompile(`stream\r?\n`)
	var text strings.Builder
	for _, loc := range re.FindAllIndex(raw, -1) {
		end := bytes.Index(raw[loc[1]:], []byte("endstream"))
		if end < 0 {
			continue
		}
		zr, err := zlib.NewReader(bytes.NewReader(raw[loc[1] : loc[1]+end]))
		if err != nil {
			continue
		}
		io.Copy(&text, zr)
		zr.Close()
	}
	s := text.String()
	missing := 0
	for _, want := range []string{
		"PWAMS Summary Report", "Dashboard", "Total Beneficiaries",
		"Total Care Provided", "Donations", "Total Amount", "154375.50",
		"Aid Requests", "Total Requests", "Pending", "Approved",
		"Rejected", "Cancelled", "Generated",
	} {
		if !strings.Contains(s, want) {
			fmt.Println("MISSING:", want)
			missing++
		}
	}
	os.WriteFile("bin\\dump.txt", []byte(s), 0o644)
	if missing > 0 {
		os.Exit(1)
	}
	fmt.Println("ALL CONTENT PRESENT")
}
