package skills

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func frontmatterOrigin(fm frontmatter) string {
	return strings.TrimSpace(fm.Metadata.Cometline.Origin)
}

// preserveSelfImprovementOrigin keeps origin: self-improvement when a parent
// edit omits it. An explicit origin in the new content is left unchanged.
func preserveSelfImprovementOrigin(existing, next string) (string, error) {
	old, err := parseFrontmatter(existing)
	if err != nil {
		return next, nil
	}
	if frontmatterOrigin(old) != OriginSelfImprovement {
		return next, nil
	}
	updated, err := parseFrontmatter(next)
	if err != nil {
		return "", err
	}
	if frontmatterOrigin(updated) != "" {
		return next, nil
	}
	return withOrigin(next, OriginSelfImprovement)
}

// withOrigin sets metadata.cometline.origin and leaves the markdown body intact.
func withOrigin(content, origin string) (string, error) {
	front, body, err := splitSkillDocument(content)
	if err != nil {
		return "", err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(front), &doc); err != nil {
		return "", err
	}
	root := mappingNode(&doc)
	if root == nil {
		return "", fmt.Errorf("skill frontmatter is not a mapping")
	}
	meta := ensureMapping(root, "metadata")
	comet := ensureMapping(meta, "cometline")
	setScalar(comet, "origin", origin)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		_ = enc.Close()
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	yamlText := strings.TrimRight(buf.String(), "\n")
	if body == "" {
		return "---\n" + yamlText + "\n---\n", nil
	}
	return "---\n" + yamlText + "\n---\n" + body, nil
}

func splitSkillDocument(raw string) (front, body string, err error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return "", "", fmt.Errorf("missing YAML frontmatter")
	}
	rest := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", fmt.Errorf("unterminated YAML frontmatter")
	}
	front = rest[:idx]
	body = rest[idx+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	return front, body, nil
}

func mappingNode(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil
		}
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	return doc
}

func ensureMapping(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key && parent.Content[i+1].Kind == yaml.MappingNode {
			return parent.Content[i+1]
		}
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
	)
	return parent.Content[len(parent.Content)-1]
}

func setScalar(parent *yaml.Node, key, value string) {
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1] = scalar
			return
		}
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		scalar,
	)
}
