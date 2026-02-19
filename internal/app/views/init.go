package views

import (
	"github.com/antonkarounis/stoic/internal/platform/web"
)

var registry *web.TemplateRegistry

func Init(m *web.TemplateRegistry) {
	registry = m
}
