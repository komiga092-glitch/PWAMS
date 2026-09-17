package main
import (
"encoding/json"
"fmt"
"os"
)
func main() {
for _, f := range []string{"en", "ta", "si"} {
data, err := os.ReadFile("internal/i18n/locales/" + f + ".json")
if err != nil { fmt.Println(f, "READ ERR", err); continue }
var m map[string]string
if err := json.Unmarshal(data, &m); err != nil { fmt.Println(f, "PARSE ERR:", err); continue }
fmt.Printf("%s OK: %d keys\n", f, len(m))
}
}