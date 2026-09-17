package main

import (
"encoding/json"
"fmt"
"os"
"sort"
"strings"
)

func main() {
en, err := os.ReadFile("internal/i18n/locales/en.json")
if err != nil {
fmt.Println(err)
os.Exit(1)
}
var m map[string]string
if err := json.Unmarshal(en, &m); err != nil {
fmt.Println(err)
os.Exit(1)
}
prefix := ""
if len(os.Args) > 1 {
prefix = os.Args[1]
}
keys := make([]string, 0, len(m))
for k := range m {
if strings.HasPrefix(k, prefix) {
keys = append(keys, k)
}
}
sort.Strings(keys)
for _, k := range keys {
fmt.Printf("%s = %s\n", k, m[k])
}
}
