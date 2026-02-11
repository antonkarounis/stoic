package handlers

import (
	"github.com/antonkarounis/stoic/internal/platform/web"
)

var manager *web.TemplateManager

func Init(m *web.TemplateManager) {
	manager = m
}
