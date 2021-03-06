package fastly

import (
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

type HerokuServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingHeroku() ServiceAttributeDefinition {
	return &HerokuServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key: "logging_heroku",
		},
	}
}

func (h *HerokuServiceAttributeHandler) Process(d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
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

	removeHerokuLogging := ols.Difference(nls).List()
	addHerokuLogging := nls.Difference(ols).List()

	// DELETE old Heroku logging endpoints.
	for _, oRaw := range removeHerokuLogging {
		of := oRaw.(map[string]interface{})
		opts := buildDeleteHeroku(of, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Heroku logging endpoint removal opts: %#v", opts)

		if err := deleteHeroku(conn, opts); err != nil {
			return err
		}
	}

	// POST new/updated Heroku logging endpoints.
	for _, nRaw := range addHerokuLogging {
		lf := nRaw.(map[string]interface{})
		opts := buildCreateHeroku(lf, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Heroku logging addition opts: %#v", opts)

		if err := createHeroku(conn, opts); err != nil {
			return err
		}
	}

	return nil
}

func (h *HerokuServiceAttributeHandler) Read(d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh Heroku.
	log.Printf("[DEBUG] Refreshing Heroku logging endpoints for (%s)", d.Id())
	herokuList, err := conn.ListHerokus(&gofastly.ListHerokusInput{
		Service: d.Id(),
		Version: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Heroku logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	ell := flattenHeroku(herokuList)

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting Heroku logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func createHeroku(conn *gofastly.Client, i *gofastly.CreateHerokuInput) error {
	_, err := conn.CreateHeroku(i)
	return err
}

func deleteHeroku(conn *gofastly.Client, i *gofastly.DeleteHerokuInput) error {
	err := conn.DeleteHeroku(i)

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

func flattenHeroku(herokuList []*gofastly.Heroku) []map[string]interface{} {
	var res []map[string]interface{}
	for _, ll := range herokuList {
		// Convert Heroku logging to a map for saving to state.
		nll := map[string]interface{}{
			"name":               ll.Name,
			"token":              ll.Token,
			"url":                ll.URL,
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

		res = append(res, nll)
	}

	return res
}

func buildCreateHeroku(herokuMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateHerokuInput {
	df := herokuMap.(map[string]interface{})

	return &gofastly.CreateHerokuInput{
		Service:           serviceID,
		Version:           serviceVersion,
		Name:              gofastly.NullString(df["name"].(string)),
		Token:             gofastly.NullString(df["token"].(string)),
		URL:               gofastly.NullString(df["url"].(string)),
		Format:            gofastly.NullString(df["format"].(string)),
		FormatVersion:     gofastly.Uint(uint(df["format_version"].(int))),
		Placement:         gofastly.NullString(df["placement"].(string)),
		ResponseCondition: gofastly.NullString(df["response_condition"].(string)),
	}
}

func buildDeleteHeroku(herokuMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteHerokuInput {
	df := herokuMap.(map[string]interface{})

	return &gofastly.DeleteHerokuInput{
		Service: serviceID,
		Version: serviceVersion,
		Name:    df["name"].(string),
	}
}

func (h *HerokuServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				// Required fields
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The unique name of the Heroku logging endpoint.",
				},

				"token": {
					Type:        schema.TypeString,
					Required:    true,
					Sensitive:   true,
					Description: "The token to use for authentication (https://www.heroku.com/docs/customer-token-authentication-token/).",
				},

				"url": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The url to stream logs to.",
				},

				// Optional fields
				"format": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Apache-style string or VCL variables to use for log formatting.",
				},

				"format_version": {
					Type:         schema.TypeInt,
					Optional:     true,
					Default:      2,
					Description:  "The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).",
					ValidateFunc: validateLoggingFormatVersion(),
				},

				"placement": {
					Type:         schema.TypeString,
					Optional:     true,
					Description:  "Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.",
					ValidateFunc: validateLoggingPlacement(),
				},

				"response_condition": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute.",
				},
			},
		},
	}
	return nil
}
