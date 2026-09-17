package main

import (
"encoding/json"
"fmt"
"os"
)

func load(p string) map[string]string {
b, err := os.ReadFile(p)
if err != nil {
panic(err)
}
m := map[string]string{}
if err := json.Unmarshal(b, &m); err != nil {
fmt.Println("PARSE FAIL", p, err)
os.Exit(1)
}
return m
}

func main() {
en := load("internal/i18n/locales/en.json")
ta := load("internal/i18n/locales/ta.json")
si := load("internal/i18n/locales/si.json")
fmt.Printf("en=%d ta=%d si=%d\n", len(en), len(ta), len(si))
missing := 0
for k := range en {
if _, ok := ta[k]; !ok {
fmt.Println("ta missing:", k)
missing++
}
if _, ok := si[k]; !ok {
fmt.Println("si missing:", k)
missing++
}
}
for k := range ta {
if _, ok := en[k]; !ok {
fmt.Println("en missing:", k)
missing++
}
}
for k := range si {
if _, ok := en[k]; !ok {
fmt.Println("en missing:", k)
missing++
}
}
fmt.Println("total missing:", missing)
}
