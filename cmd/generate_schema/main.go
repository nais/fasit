package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/nais/fasit/pkg/graph/model"
)

func main() {
	out := "./schema/jsonschema/feature.json"
	flag.StringVar(&out, "out", out, "output file")
	flag.Parse()

	r := &jsonschema.Reflector{}
	r.DoNotReference = true
	r.BaseSchemaID = "https://fasit.nais.io/schema"
	r.AddGoComments("github.com/nais/fasit", "./pkg/graph/model/")
	schema := r.Reflect(model.FeatureYAML{})

	b, err := schema.MarshalJSON()
	if err != nil {
		log.Fatal(err)
	}

	v := map[string]any{}
	_ = json.Unmarshal(b, &v)
	b, _ = json.MarshalIndent(v, "", "\t")

	if err := os.WriteFile(out, b, 0o664); err != nil {
		log.Fatal(err)
	}
}
