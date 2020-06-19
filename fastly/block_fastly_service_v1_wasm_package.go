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
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"filename": {
					Type:          schema.TypeString,
					Optional:      true,
				},
				"source_code_hash": {
					Type:     schema.TypeString,
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
	serviceID := d.Id()
	ol, nl := d.GetChange(h.GetKey())

	if ol == nil {
		ol = new(schema.Set)
	}
	if nl == nil {
		nl = new(schema.Set)
	}

	ols := ol.(*schema.Set)
	nls := nl.(*schema.Set)

	removeLogglyLogging := ols.Difference(nls).List()
	addLogglyLogging := nls.Difference(ols).List()

	// DELETE old Loggly logging endpoints.
	for _, oRaw := range removeLogglyLogging {
		of := oRaw.(map[string]interface{})
		opts := buildDeleteLoggly(of, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Loggly logging endpoint removal opts: %#v", opts)

		if err := deleteLoggly(conn, opts); err != nil {
			return err
		}
	}

	// POST new/updated Loggly logging endpoints.
	for _, nRaw := range addLogglyLogging {
		lf := nRaw.(map[string]interface{})
		opts := buildCreateLoggly(lf, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Loggly logging addition opts: %#v", opts)

		if err := createLoggly(conn, opts); err != nil {
			return err
		}
	}

	return nil
}

func (h *WASMPackageServiceAttributeHandler) Read(d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh Loggly.
	log.Printf("[DEBUG] Refreshing Loggly logging endpoints for (%s)", d.Id())
	logglyList, err := conn.ListLoggly(&gofastly.ListLogglyInput{
		Service: d.Id(),
		Version: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Loggly logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	ell := flattenLoggly(logglyList)

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting Loggly logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}


func createWASMPackage(conn *gofastly.Client, i *gofastly.UpdateWASMPackageInput) error {
	_, err := conn.CreateLoggly(i)
	return err
}

func deleteWASMPackage(conn *gofastly.Client, i *gofastly.DeleteLogglyInput) error {
	err := conn.DeleteLoggly(i)

	errRes, ok := err.(*gofastly.HTTPError)
	if !ok {
		return err
	}

	// 404 response codes don't result in an error propagating because a 404 could
	// indicate that a resource was deleted elsewhere.
	if !errRes.IsNotFound() {
		return err
	}

	return nil
}

func flattenWASMPackage(logglyList []*gofastly.Loggly) []map[string]interface{} {
	var lsl []map[string]interface{}
	for _, ll := range logglyList {
		// Convert Loggly logging to a map for saving to state.
		nll := map[string]interface{}{
			"name":               ll.Name,
			"token":              ll.Token,
			"format":             ll.Format,
			"format_version":     ll.FormatVersion,
			"placement":          ll.Placement,
			"response_condition": ll.ResponseCondition,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range nll {
			if v == "" {
				delete(nll, k)
			}
		}

		lsl = append(lsl, nll)
	}

	return lsl
}

func buildCreateWASMPackage(logglyMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateLogglyInput {
	df := logglyMap.(map[string]interface{})

	return &gofastly.CreateLogglyInput{
		Service:           serviceID,
		Version:           serviceVersion,
		Name:              gofastly.NullString(df["name"].(string)),
		Token:             gofastly.NullString(df["token"].(string)),
		Format:            gofastly.NullString(df["format"].(string)),
		FormatVersion:     gofastly.Uint(uint(df["format_version"].(int))),
		Placement:         gofastly.NullString(df["placement"].(string)),
		ResponseCondition: gofastly.NullString(df["response_condition"].(string)),
	}
}

func buildDeleteWASMPackage(logglyMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteLogglyInput {
	df := logglyMap.(map[string]interface{})

	return &gofastly.DeleteLogglyInput{
		Service: serviceID,
		Version: serviceVersion,
		Name:    df["name"].(string),
	}
}


