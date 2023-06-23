package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	feature "github.com/nais/fasit/pkg/feature"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func main() {
	var oldFeature *feature.Feature
	app := &cli.App{
		Name:  "convert",
		Usage: "Converts a feature from v1 to v2",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "feature-only",
				Usage: "Only print the feature, not the chart and other changes",
			},
		},
		Before: func(c *cli.Context) error {
			b, err := os.ReadFile(filepath.Join("features", c.Args().Get(0)+".yaml"))
			if err != nil {
				return err
			}

			if err := yaml.Unmarshal(b, &oldFeature); err != nil {
				return err
			}
			oldFeature.Name = c.Args().Get(0)
			return nil
		},
		Action: func(c *cli.Context) error {
			if !c.Bool("feature-only") {
				chart(oldFeature)
			}
			featurev2(oldFeature, !c.Bool("feature-only"))
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func featurev2(f *feature.Feature, debug bool) {
	fy := feature.FeatureV2(f, debug)
	db, _ := json.Marshal(f.DependsOn)
	json.Unmarshal(db, &fy.Dependencies)

	if len(f.AutoInstall) > 0 && debug {
		fmt.Println("WARNING: 'autoInstall' is not supported in v2")
	}

	printYaml(fy)
}

func chart(f *feature.Feature) {
	fmt.Println("Chart.yaml:")
	fmt.Println("'chart' is removed in v2")
	cy := map[string]any{
		"name":    f.Name,
		"version": f.Version,
	}
	if f.Description != "" {
		cy["description"] = f.Description
	}
	if f.Source != "" {
		cy["sources"] = []string{f.Source}
	}

	if f.Repo != "" {
		fmt.Println("ERROR: features using 'repo' are not supported in v2")
	}
	printYaml(cy)
	fmt.Println("\n---\n\nFeature.yaml:")
	fmt.Println()
}

func printYaml(v any) {
	m := yaml.NewEncoder(os.Stdout)
	m.SetIndent(2)
	m.Encode(v)
}
