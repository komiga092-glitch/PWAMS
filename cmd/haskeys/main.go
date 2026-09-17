package main

// haskeys reports whether the given translation keys exist, and whether every
// locale defines them. It prints "ok <key>" or "MISSING <lang> <key>" so gaps
// surfaced while threading i18n through the templates can be fixed.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

func load(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		panic(path + ": " + err.Error())
	}
	return m
}

func main() {
	langs := map[string]map[string]string{}
	for _, l := range []string{"en", "ta", "si"} {
		langs[l] = load("internal/i18n/locales/" + l + ".json")
	}

	// With no args, compare key sets across locales and report gaps.
	if len(os.Args) == 1 {
		gaps := 0
		keys := make([]string, 0, len(langs["en"]))
		for k := range langs["en"] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, l := range []string{"ta", "si"} {
				if v, ok := langs[l][k]; !ok || v == "" {
					fmt.Printf("MISSING %s %s\n", l, k)
					gaps++
				}
			}
		}
		fmt.Printf("en=%d ta=%d si=%d gaps=%d\n",
			len(langs["en"]), len(langs["ta"]), len(langs["si"]), gaps)
		if gaps > 0 {
			os.Exit(1)
		}
		return
	}

	bad := 0
	for _, key := range os.Args[1:] {
		for _, l := range []string{"en", "ta", "si"} {
			if v, ok := langs[l][key]; !ok || v == "" {
				fmt.Printf("MISSING %s %s\n", l, key)
				bad++
			}
		}
		if v, ok := langs["en"][key]; ok {
			fmt.Printf("ok %-32s en=%q\n", key, v)
		}
	}
	if bad > 0 {
		os.Exit(1)
	}
}
