package fastly

import (
	gofastly "github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// ServiceAttributeDefinition provides an interface for service attributes.
// We compose a service resource out of attribute objects to allow us to construct both the VCL and WASM service
// resources from common components
type ServiceAttributeDefinition interface {
	// Register add the attribute to the resource schema
	Register(d *schema.Resource, serviceType string) error

	// Read refreshes the attribute state against the Fastly API
	Read(d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error

	// Process creates or updates the attribute against the Fastly API
	Process(d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error

	// HasChange returns whether the state of the attribute has changed against Terraform stored state
	HasChange(d *schema.ResourceData) bool

	// MustProcess returns whether we must Process the resource (usually HasChange==true but allowing exceptions)
	MustProcess(d *schema.ResourceData, initialVersion bool) bool
}

// DefaultServiceAttributeHandler provides a base implementation for ServiceAttributeDefinition
type DefaultServiceAttributeHandler struct {
	schema *schema.Schema
	key    string
}

// GetKey is provided since most attributes will just use their private "key" for interacting with the service
func (h *DefaultServiceAttributeHandler) GetKey() string {
	return h.key
}

func (h *DefaultServiceAttributeHandler) HasChange(d *schema.ResourceData) bool {
	return d.HasChange(h.key)
}

func (h *DefaultServiceAttributeHandler) MustProcess(d *schema.ResourceData, initialVersion bool) bool {
	return h.HasChange(d)
}

func (h *DefaultServiceAttributeHandler) OptionalMapKeyToString(m map[string]interface{}, k string) string {
	v, ok := m[k]
	if ok {
		return v.(string)
	} else {
		return ""
	}
}
