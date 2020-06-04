package fastly

import (
	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

type ServiceDefinition interface {
	GetType() string
	GetAttributeHandler() []AttributeHandler
}

type AttributeHandler interface {
	GetKey() string
	GetSchema() *schema.Schema
	Read(d *schema.ResourceData, conn *fastly.Client, s *fastly.ServiceDetail) error
	Process(d *schema.ResourceData, latestVersion int, conn *fastly.Client) error
}

type DefaultServiceDefinition struct {
	Attributes []AttributeHandler
	Type       string
}

func (d *DefaultServiceDefinition) GetType() string {
	return d.Type
}

func (d *DefaultServiceDefinition) GetAttributeHandler() []AttributeHandler {
	return d.Attributes
}

type DefaultAttributeHandler struct {
	schema *schema.Schema
	key    string
}

func (h *DefaultAttributeHandler) GetSchema() *schema.Schema {
	return h.schema
}

func (h *DefaultAttributeHandler) GetKey() string {
	return h.key
}
