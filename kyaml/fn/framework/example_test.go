// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package framework_test

import (
	"bytes"
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ExampleCommand_modify implements a function that sets an annotation on each resource.
// The annotation value is configured via a flag value parsed from
// ResourceList.functionConfig.data
func ExampleCommand_modify() {
	// configure the annotation value using a flag parsed from
	// ResourceList.functionConfig.data.value
	var value string
	cmd := framework.Command(nil, func(items []*yaml.RNode) ([]*yaml.RNode, error) {
		for i := range items {
			// set the annotation on each resource item
			if err := items[i].PipeE(yaml.SetAnnotation("value", value)); err != nil {
				return nil, err
			}
		}
		return items, nil
	})
	cmd.Flags().StringVar(&value, "value", "", "annotation value")

	// for testing purposes only -- normally read from stdin when Executing
	cmd.SetIn(bytes.NewBufferString(`
apiVersion: config.kubernetes.io/v1alpha1
kind: ResourceList
# items are provided as nodes
items:
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: foo
- apiVersion: v1
  kind: Service
  metadata:
    name: foo
# functionConfig is parsed into flags by framework.Command
functionConfig:
  apiVersion: v1
  kind: ConfigMap
  data:
    value: baz
`))
	// run the command
	if err := cmd.Execute(); err != nil {
		panic(err)
	}

	// Output:
	// apiVersion: config.kubernetes.io/v1alpha1
	// kind: ResourceList
	// items:
	// - apiVersion: apps/v1
	//   kind: Deployment
	//   metadata:
	//     name: foo
	//     annotations:
	//       value: 'baz'
	// - apiVersion: v1
	//   kind: Service
	//   metadata:
	//     name: foo
	//     annotations:
	//       value: 'baz'
	// functionConfig:
	//   apiVersion: v1
	//   kind: ConfigMap
	//   data:
	//     value: baz
}

// ExampleCommand_generateReplace generates a resource from a functionConfig.
// If the resource already exist s, it replaces the resource with a new copy.
func ExampleCommand_generateReplace() {
	// function API definition which will be parsed from the ResourceList.functionConfig
	// read from stdin
	type Spec struct {
		Name string `yaml:"name,omitempty"`
	}
	type ExampleServiceGenerator struct {
		Spec Spec `yaml:"spec,omitempty"`
	}
	functionConfig := &ExampleServiceGenerator{}

	// function implementation -- generate a Service resource
	cmd := framework.Command(functionConfig, func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		var newNodes []*yaml.RNode
		for i := range nodes {
			meta, err := nodes[i].GetMeta()
			if err != nil {
				return nil, err
			}

			// something we already generated, remove it from the list so we regenerate it
			if meta.Name == functionConfig.Spec.Name &&
				meta.Kind == "Service" &&
				meta.APIVersion == "v1" {
				continue
			}
			newNodes = append(newNodes, nodes[i])
		}

		// generate the resource
		n, err := yaml.Parse(fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
`, functionConfig.Spec.Name))
		if err != nil {
			return nil, err
		}
		return append(newNodes, n), nil
	})

	// for testing purposes only -- normally read from stdin when Executing
	cmd.SetIn(bytes.NewBufferString(`
apiVersion: config.kubernetes.io/v1alpha1
kind: ResourceList
# items are provided as nodes
items:
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: foo
# functionConfig is parsed into flags by framework.Command
functionConfig:
  apiVersion: example.com/v1alpha1
  kind: ExampleServiceGenerator
  spec:
    name: bar
`))

	// run the command
	if err := cmd.Execute(); err != nil {
		panic(err)
	}

	// Output:
	// apiVersion: config.kubernetes.io/v1alpha1
	// kind: ResourceList
	// items:
	// - apiVersion: apps/v1
	//   kind: Deployment
	//   metadata:
	//     name: foo
	// - apiVersion: v1
	//   kind: Service
	//   metadata:
	//     name: bar
	// functionConfig:
	//   apiVersion: example.com/v1alpha1
	//   kind: ExampleServiceGenerator
	//   spec:
	//     name: bar
}

// ExampleCommand_generateUpdate generates a resource, updating the previously generated
// copy rather than replacing it.
//
// Note: This will keep manual edits to the previously generated copy.
func ExampleCommand_generateUpdate() {
	// function API definition which will be parsed from the ResourceList.functionConfig
	// read from stdin
	type Spec struct {
		Name        string            `yaml:"name,omitempty"`
		Annotations map[string]string `yaml:"annotations,omitempty"`
	}
	type ExampleServiceGenerator struct {
		Spec Spec `yaml:"spec,omitempty"`
	}
	functionConfig := &ExampleServiceGenerator{}

	// function implementation -- generate or update a Service resource
	cmd := framework.Command(functionConfig, func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		var found bool
		for i := range nodes {
			meta, err := nodes[i].GetMeta()
			if err != nil {
				return nil, err
			}

			// something we already generated, reconcile it to make sure it matches what
			// is specified by the functionConfig
			if meta.Name == functionConfig.Spec.Name &&
				meta.Kind == "Service" &&
				meta.APIVersion == "v1" {
				// set some values
				for k, v := range functionConfig.Spec.Annotations {
					err := nodes[i].PipeE(yaml.SetAnnotation(k, v))
					if err != nil {
						return nil, err
					}
				}
				found = true
				break
			}
		}
		if found {
			return nodes, nil
		}

		// generate the resource if not found
		n, err := yaml.Parse(fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
`, functionConfig.Spec.Name))
		for k, v := range functionConfig.Spec.Annotations {
			err := n.PipeE(yaml.SetAnnotation(k, v))
			if err != nil {
				return nil, err
			}
		}
		nodes = append(nodes, n)
		if err != nil {
			return nil, err
		}

		return nodes, nil
	})

	// for testing purposes only -- normally read from stdin when Executing
	cmd.SetIn(bytes.NewBufferString(`
apiVersion: config.kubernetes.io/v1alpha1
kind: ResourceList
# items are provided as nodes
items:
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: foo
- apiVersion: v1
  kind: Service
  metadata:
    name: bar
# functionConfig is parsed into flags by framework.Command
functionConfig:
  apiVersion: example.com/v1alpha1
  kind: ExampleServiceGenerator
  spec:
    name: bar
    annotations:
      a: b
`))

	// run the command
	if err := cmd.Execute(); err != nil {
		panic(err)
	}

	// Output:
	// apiVersion: config.kubernetes.io/v1alpha1
	// kind: ResourceList
	// items:
	// - apiVersion: apps/v1
	//   kind: Deployment
	//   metadata:
	//     name: foo
	// - apiVersion: v1
	//   kind: Service
	//   metadata:
	//     name: bar
	//     annotations:
	//       a: 'b'
	// functionConfig:
	//   apiVersion: example.com/v1alpha1
	//   kind: ExampleServiceGenerator
	//   spec:
	//     name: bar
	//     annotations:
	//       a: b
}

// ExampleCommand_validate validates that all Deployment resources have the replicas field set.
// If any Deployments do not contain spec.replicas, then the function will return Results
// which will be set on ResourceList.results
func ExampleCommand_validate() {
	cmd := framework.Command(nil, func(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
		// validation results
		var validationResults []framework.Item

		// validate that each Deployment resource has spec.replicas set
		for i := range nodes {
			// only check Deployment resources
			meta, err := nodes[i].GetMeta()
			if err != nil {
				return nil, err
			}
			if meta.Kind != "Deployment" {
				continue
			}

			// lookup replicas field
			r, err := nodes[i].Pipe(yaml.Lookup("spec", "replicas"))
			if err != nil {
				return nil, err
			}

			// check replicas not specified
			if r != nil {
				continue
			}
			validationResults = append(validationResults, framework.Item{
				Severity:    framework.Error,
				Message:     "missing replicas",
				ResourceRef: meta,
				Field: framework.Field{
					Path:           "spec.field",
					SuggestedValue: "1",
				},
			})
		}

		// framework will only consider Results an error if it has at least 1 item
		return nodes, framework.Result{
			Name:  "replicas-validator",
			Items: validationResults,
		}
	})

	// for testing purposes only -- normally read from stdin when Executing
	cmd.SetIn(bytes.NewBufferString(`
apiVersion: config.kubernetes.io/v1alpha1
kind: ResourceList
# items are provided as nodes
items:
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: foo
`))

	// run the command
	if err := cmd.Execute(); err != nil {
		// normally exit 1 here
	}

	// Output:
	// apiVersion: config.kubernetes.io/v1alpha1
	// kind: ResourceList
	// items:
	// - apiVersion: apps/v1
	//   kind: Deployment
	//   metadata:
	//     name: foo
	// results:
	//   name: replicas-validator
	//   items:
	//   - message: missing replicas
	//     severity: error
	//     resourceRef:
	//       apiVersion: apps/v1
	//       kind: Deployment
	//       metadata:
	//         name: foo
	//     field:
	//       path: spec.field
	//       suggestedValue: "1"
}
