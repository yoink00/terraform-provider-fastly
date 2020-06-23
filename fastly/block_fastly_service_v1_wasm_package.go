package fastly

import (
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

type WASMPackageServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceWASMPackage() ServiceAttributeDefinition {
	return &WASMPackageServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key: "package",
		},
	}
}

func (h *WASMPackageServiceAttributeHandler) Register(s *schema.Resource, serviceType string) error {
	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		MaxItems: 1,
		MinItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"filename": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"source_code_hash": {
					Type:     schema.TypeString,
					Optional: true,
					Computed: true,
				},
				"source_code_size": {
					Type:     schema.TypeInt,
					Computed: true,
				},
			},
		},
	}
	return nil
}

func (h *WASMPackageServiceAttributeHandler) Process(d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {

	if d.HasChange(h.GetKey()) {
		if v, ok := d.GetOk(h.GetKey()); ok {
			// Schema guarantees one package block
			wasmPackage := v.(*schema.Set).List()[0].(map[string]interface{})
			packageFilename := wasmPackage["filename"].(string)

			err := updateWASMPackage(conn, &gofastly.UpdateWASMPackageInput{
				Service:     d.Id(),
				Version:     latestVersion,
				PackagePath: packageFilename,
			})
			if err != nil {
				return fmt.Errorf("Error modifying WASM Package %s: %s", d.Id(), err)
			}
		}
	}
	return nil
}

func (h *WASMPackageServiceAttributeHandler) Read(d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {

	log.Printf("[DEBUG] Refreshing WASM package for (%s)", d.Id())
	wasmPackage, err := conn.GetWASMPackage(&gofastly.GetWASMPackageInput{
		Service: d.Id(),
		Version: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up WASM Package for (%s), version (%v): %v", d.Id(), s.ActiveVersion.Number, err)
	}

	wp := flattenWASMPackage(wasmPackage)
	if err := d.Set(h.GetKey(), wp); err != nil {
		log.Printf("[WARN] Error setting WASM Package for (%s): %s", d.Id(), err)
	}

	return nil
}

func updateWASMPackage(conn *gofastly.Client, i *gofastly.UpdateWASMPackageInput) error {
	_, err := conn.UpdateWASMPackage(i)
	return err
}

func flattenWASMPackage(wasmPackage *gofastly.WASMPackage) []map[string]interface{} {
	var wp []map[string]interface{}

	// Convert WASM Package to a map for saving to state.
	wp = append(wp, map[string]interface{}{
		"source_code_hash": wasmPackage.Metadata.HashSum,
		"source_code_size": wasmPackage.Metadata.Size,
	})

	return wp
}
