/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/org/cmd/helpers"
	"k8s.io/test-infra/prow/config/org"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
)

type options struct {
	orgs        helpers.FlagMap
	mergeTeams  bool
	ignoreTeams bool
}

func main() {
	o := options{orgs: helpers.FlagMap{}}
	flag.Var(o.orgs, "org-part", "Each instance adds an org-name=org.yaml part")
	flag.BoolVar(&o.mergeTeams, "merge-teams", false, "Merge team-name/team.yaml files in each org.yaml dir")
	flag.BoolVar(&o.ignoreTeams, "ignore-teams", false, "Never configure teams")
	flag.Parse()

	for _, a := range flag.Args() {
		logrus.Print("Extra", a)
		o.orgs.Set(a)
	}

	if o.mergeTeams && o.ignoreTeams {
		logrus.Fatal("--merge-teams xor --ignore-teams, not both")
	}

	cfg, err := loadOrgs(o)
	if err != nil {
		logrus.Fatalf("Failed to load orgs: %v", err)
	}
	pc := org.FullConfig{
		Orgs: cfg,
	}
	out, err := yaml.Marshal(pc)
	if err != nil {
		logrus.Fatalf("Failed to marshal orgs: %v", err)
	}
	fmt.Println(string(out))
}

func loadOrgs(o options) (map[string]org.Config, error) {
	config := map[string]org.Config{}
	for name, path := range o.orgs {
		cfg, err := helpers.UnmarshalPathToOrgConfig(path)
		if err != nil {
			return nil, fmt.Errorf("error in %s: %v", path, err)
		}
		switch {
		case o.ignoreTeams:
			cfg.Teams = nil
		case o.mergeTeams:
			if cfg.Teams == nil {
				cfg.Teams = map[string]org.Team{}
			}
			prefix := filepath.Dir(path)
			err := filepath.Walk(prefix, func(path string, info os.FileInfo, err error) error {
				switch {
				case path == prefix:
					return nil // Skip base dir
				case info.IsDir() && filepath.Dir(path) != prefix:
					logrus.Infof("Skipping %s and its children", path)
					return filepath.SkipDir // Skip prefix/foo/bar/ dirs
				case !info.IsDir() && filepath.Dir(path) == prefix:
					return nil // Ignore prefix/foo files
				case filepath.Base(path) == "teams.yaml":
					teamCfg, err := helpers.UnmarshalPathToOrgConfig(path)
					if err != nil {
						return fmt.Errorf("error in %s: %v", path, err)
					}

					for name, team := range teamCfg.Teams {
						cfg.Teams[name] = team
					}
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("merge teams %s: %v", path, err)
			}
		}
		config[name] = *cfg
	}
	return config, nil
}
