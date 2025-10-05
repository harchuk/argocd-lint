package manifest

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/argocd-lint/argocd-lint/pkg/types"
	"gopkg.in/yaml.v3"
)

// Manifest represents a parsed Kubernetes document of interest.
type Manifest struct {
	FilePath      string
	DocumentIndex int
	Kind          string
	APIVersion    string
	Name          string
	Namespace     string
	Line          int
	Column        int
	MetadataLine  int
	Object        map[string]interface{}
	Node          *yaml.Node
}

// Parser converts YAML/JSON files into manifest structures.
type Parser struct{}

// ParseFile parses the provided manifest file and returns supported resources.
func (Parser) ParseFile(path string) ([]*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(false)

	var manifests []*Manifest
	idx := 0
	for {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode manifest: %w", err)
		}
		if node.Kind == 0 {
			continue
		}
		m, err := parseNode(path, idx, &node)
		if err != nil {
			return nil, err
		}
		if m != nil {
			manifests = append(manifests, m)
		}
		idx++
	}
	return manifests, nil
}

func parseNode(path string, index int, node *yaml.Node) (*Manifest, error) {
	var obj map[string]interface{}
	if err := node.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode node to map: %w", err)
	}
	kind := getString(obj["kind"])
	apiVersion := getString(obj["apiVersion"])
	if !isSupported(kind, apiVersion) {
		return nil, nil
	}
	metadata := getMap(obj["metadata"])
	name := getString(metadata["name"])
	namespace := getString(metadata["namespace"])

	m := &Manifest{
		FilePath:      path,
		DocumentIndex: index,
		Kind:          kind,
		APIVersion:    apiVersion,
		Name:          name,
		Namespace:     namespace,
		Line:          node.Line,
		Column:        node.Column,
		MetadataLine:  findLine(node, []string{"metadata", "name"}),
		Object:        obj,
		Node:          node,
	}
	return m, nil
}

func isSupported(kind, apiVersion string) bool {
	switch kind {
	case string(types.ResourceKindApplication), string(types.ResourceKindApplicationSet), string(types.ResourceKindAppProject):
		return apiVersion == "argoproj.io/v1alpha1"
	default:
		return false
	}
}

func getString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getMap(v interface{}) map[string]interface{} {
	if v == nil {
		return map[string]interface{}{}
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func findLine(root *yaml.Node, path []string) int {
	if root == nil {
		return 0
	}
	if root.Kind != yaml.DocumentNode {
		// root should be document; adjust if not
		if root.Kind == yaml.MappingNode {
			return findLineMapping(root, path)
		}
		return root.Line
	}
	if len(root.Content) == 0 {
		return root.Line
	}
	return findLineMapping(root.Content[0], path)
}

func findLineMapping(node *yaml.Node, path []string) int {
	if node == nil {
		return 0
	}
	if node.Kind != yaml.MappingNode {
		return node.Line
	}
	if len(path) == 0 {
		return node.Line
	}
	key := path[0]
	for i := 0; i < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		if k.Value == key {
			if len(path) == 1 {
				return v.Line
			}
			return findLineMapping(v, path[1:])
		}
	}
	return node.Line
}
