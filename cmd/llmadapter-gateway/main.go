package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/codewandler/llmadapter/adapterconfig"
	"github.com/codewandler/llmadapter/gatewayserver"
)

func main() {
	inspectConfigFlag := flag.Bool("inspect-config", false, "print resolved gateway config metadata as JSON and exit")
	flag.Parse()

	cfg, err := adapterconfig.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if err := adapterconfig.Validate(cfg); err != nil {
		log.Fatal(err)
	}
	if *inspectConfigFlag {
		inspection, err := adapterconfig.InspectConfig(cfg)
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(inspection); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := gatewayserver.ListenAndServe(cfg); err != nil {
		log.Fatal(err)
	}
}
