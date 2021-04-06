package envoy

import (
	"embed"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/int128/kubectl-external-forward/pkg/tunnel"
)

//go:embed template/*
var configTemplateDir embed.FS

var configTemplate = template.Must(template.ParseFS(configTemplateDir, "template/*"))

type configTemplateContext struct {
	Tunnels []tunnel.Tunnel
}

func NewConfig(tunnels []tunnel.Tunnel) (string, error) {
	c := configTemplateContext{Tunnels: tunnels}
	var s strings.Builder
	if err := configTemplate.ExecuteTemplate(&s, "envoy.yaml", c); err != nil {
		return "", fmt.Errorf("template error: %w", err)
	}
	return s.String(), nil
}
