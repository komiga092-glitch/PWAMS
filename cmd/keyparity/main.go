package main

import (
"encoding/json"
"fmt"
"os"
"sort"
)

func load(p string) map[string]string {
b, err := os.ReadFile(p)
if err != nil {
panic(err)
}
m := map[string]string{}
if err := json.Unmarshal(b, &m); err != nil {
panic(err)
}
return m
}

func main() {
en := load("internal/i18n/locales/en.json")
for _, l := range []string{"ta", "si"} {
m := load("internal/i18n/locales/" + l + ".json")
var miss, extra []string
for k := range en {
if _, ok := m[k]; !ok {
miss = append(miss, k)
}
}
for k := range m {
if _, ok := en[k]; !ok {
extra = append(extra, k)
}
}
sort.Strings(miss)
sort.Strings(extra)
fmt.Printf("%s: %d keys, missing=%d extra=%d\n", l, len(m), len(miss), len(extra))
for _, k := range miss {
fmt.Println("  MISSING", k)
}
for _, k := range extra {
fmt.Println("  EXTRA", k)
}
}
fmt.Printf("en: %d keys\n", len(en))
}
