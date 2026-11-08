// Command dogfood exercises the transport against a live SLC and dumps
// fixtures for parser tests. Credentials via env (populated from secrets.txt).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/transport"
)

func main() {
	port, _ := strconv.Atoi(os.Getenv("SLC_PORT"))
	tr := transport.Dial(transport.Config{
		Host: os.Getenv("SLC_HOST"), Port: port,
		Username: os.Getenv("SLC_USER"), Password: os.Getenv("SLC_PASS"),
		CommandTimeout: 30 * time.Second,
	})
	commands := map[string]string{
		"data47":  "show deviceport port 47 display data",
		"ip47":    "show deviceport port 47 display ip",
		"modem47": "show deviceport port 47 display modem",
		"data48":  "show deviceport port 48 display data",
		"ip48":    "show deviceport port 48 display ip",
		"global":  "show deviceport global",
	}
	out := map[string]string{}
	for name, cmd := range commands {
		res, err := tr.Read(context.Background(), cmd)
		if err != nil {
			fmt.Fprintln(os.Stderr, name, "ERROR:", err)
			os.Exit(1)
		}
		out[name] = res
	}
	b, _ := json.MarshalIndent(out, "", " ")
	if err := os.WriteFile("lab/fixtures.json", b, 0o600); err != nil {
		panic(err)
	}
	fmt.Println("fixtures written:", len(out), "commands")
}
