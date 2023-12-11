/*
po-test
Copyright (C) 2023 loveholidays

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU Lesser General Public
License as published by the Free Software Foundation; either
version 3 of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Lesser General Public License for more details.

You should have received a copy of the GNU Lesser General Public License
along with this program; if not, write to the Free Software Foundation,
Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

package tests

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/rulefmt"
	"golang.org/x/exp/slices"
)

var (
	mandatoryLabels      = []string{"owner", "severity"}
	mandatoryAnnotations = []string{"runbook_url", "description", "summary"}
)

type RuleGroup struct {
	Name     string         `yaml:"name"`
	Interval model.Duration `yaml:"interval,omitempty"`
	Limit    int            `yaml:"limit,omitempty"`
	Rules    []rulefmt.Rule `yaml:"rules"`
}

type Rule struct {
	Record        string            `yaml:"record,omitempty"`
	Alert         string            `yaml:"alert,omitempty"`
	Expr          string            `yaml:"expr"`
	For           model.Duration    `yaml:"for,omitempty"`
	KeepFiringFor model.Duration    `yaml:"keep_firing_for,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	Annotations   map[string]string `yaml:"annotations,omitempty"`
}

type RuleGroups struct {
	Groups []RuleGroup `yaml:"groups"`
}

type Spec struct {
	Spec RuleGroups `yaml:"spec"`
}

func getLabels(ruleDef rulefmt.Rule) []string {
	presentLabels := []string{}
	for k := range ruleDef.Labels {
		presentLabels = append(presentLabels, k)
	}
	return presentLabels
}

func getAnnotations(ruleDef rulefmt.Rule) []string {
	presentLabels := []string{}
	for k := range ruleDef.Labels {
		presentLabels = append(presentLabels, k)
	}
	return presentLabels
}

func inspectRule(ruleDef rulefmt.Rule) error {
	var errored bool
	missingLabels := []string{}
	missingAnnotations := []string{}
	presentLabels := getLabels(ruleDef)
	for _, label := range mandatoryLabels {
		if !slices.Contains(presentLabels, label) {
			missingLabels = append(missingLabels, label)
		}
	}
	for _, annotation := range mandatoryAnnotations {
		if !slices.Contains(getAnnotations(ruleDef), annotation) {
			missingAnnotations = append(missingAnnotations, annotation)
		}
	}

	if len(missingLabels) > 0 {
		fmt.Println("missing mandatory labels: ", strings.Join(missingLabels, ", "))
		errored = true
	}
	if len(missingAnnotations) > 0 {
		fmt.Println("missing mandatory annotations:  ", strings.Join(missingAnnotations, ", "))
		errored = true
	}
	if errored {
		return fmt.Errorf("missing mandatory labels/annotations")
	}
	return nil
}

func RunUnitTests(testFiles []string) error {
	var originalRules []*filenameAndData
	for _, testFile := range testFiles {
		b, err := os.ReadFile(testFile)
		if err != nil {
			return err
		}

		var unitTestInp unitTestFile
		if err := yaml.Unmarshal(b, &unitTestInp); err != nil {
			return err
		}

		for _, rulesFile := range unitTestInp.RuleFiles {
			relativeRulesFile := fmt.Sprintf("%s/%s", filepath.Dir(testFile), rulesFile)

			yamlFile, err := os.ReadFile(relativeRulesFile)
			if err != nil {
				return err
			}

			unstructured := make(map[interface{}]interface{})
			err = yaml.Unmarshal(yamlFile, &unstructured)
			if err != nil {
				return err
			}

			var structuredSpec Spec
			err = yaml.Unmarshal(yamlFile, &structuredSpec)
			if err != nil {
				return err
			}

			for _, ruleGroup := range structuredSpec.Spec.Groups {
				for _, ruleDef := range ruleGroup.Rules {
					err = inspectRule(ruleDef)
					if err != nil {
						return err
					}
					fmt.Printf("%v", ruleDef.Labels)
				}
			}
			if spec, found := unstructured["spec"]; found {
				ruleFileContentWithoutMetadata, err := yaml.Marshal(spec)
				if err != nil {
					return err
				}

				originalRules = append(originalRules, &filenameAndData{relativeRulesFile, yamlFile})

				err = os.WriteFile(relativeRulesFile, ruleFileContentWithoutMetadata, 0o600)
				if err != nil {
					return err
				}
			} else {
				log.Printf("No spec found in file %s", rulesFile)
			}
		}
	}

	promtoolArgs := append([]string{"test", "rules"}, testFiles...)
	command := exec.Command("promtool", promtoolArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		log.Printf("%s", output)
		restoreOriginalFiles(originalRules)
		return err
	}
	log.Printf("%s", output)
	restoreOriginalFiles(originalRules)
	return nil
}

func restoreOriginalFiles(rules []*filenameAndData) {
	for _, nameAndData := range rules {
		err := os.WriteFile(nameAndData.filename, nameAndData.data, 0o600)
		if err != nil {
			log.Fatalf("Failed to write file: %v", err)
		}
	}
}

type filenameAndData struct {
	filename string
	data     []byte
}

type unitTestFile struct {
	RuleFiles []string `yaml:"rule_files"`
}
