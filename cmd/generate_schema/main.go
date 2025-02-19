package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/nais/fasit/internal/graph/model"
)

func main() {
	err := genSchema("./schema/jsonschema/feature.json", model.FeatureYAML{}, "github.com/nais/fasit", "./internal/graph/model/")
	if err != nil {
		log.Fatal(err)
	}
}

func genSchema(out string, v any, pkg, pkgPath string) error {
	r := &jsonschema.Reflector{}
	r.DoNotReference = true
	r.BaseSchemaID = "https://fasit.nais.io/schema"
	if err := r.AddGoComments(pkg, pkgPath); err != nil {
		return err
	}
	schema := r.Reflect(v)

	b, err := schema.MarshalJSON()
	if err != nil {
		return err
	}

	res := map[string]any{}
	_ = json.Unmarshal(b, &res)
	b, _ = json.MarshalIndent(res, "", "\t")

	return os.WriteFile(out, b, 0o664) // #nosec G306
}
