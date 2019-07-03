package fastly

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	gofastly "github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/schema"
)

var fastlyNoServiceFoundErr = errors.New("No matching Fastly Service found")

func resourceServiceV1() *schema.Resource {
	return &schema.Resource{
		Create: resourceServiceV1Create,
		Read:   resourceServiceV1Read,
		Update: resourceServiceV1Update,
		Delete: resourceServiceV1Delete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique name for this Service",
			},

			"comment": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "Managed by Terraform",
				Description: "A personal freeform descriptive note",
			},

			"version_comment": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "A personal freeform descriptive note",
			},

			// Active Version represents the currently activated version in Fastly. In
			// Terraform, we abstract this number away from the users and manage
			// creating and activating. It's used internally, but also exported for
			// users to see.
			"active_version": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"activate": {
				Type:        schema.TypeBool,
				Description: "Conditionally prevents the Service from being activated",
				Default:     true,
				Optional:    true,
			},

			"domain": domainSchema,

			"condition": conditionSchema,

			"default_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     3600,
				Description: "The default Time-to-live (TTL) for the version",
			},

			"default_host": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The default hostname for the version",
			},

			"healthcheck": healthcheckSchema,

			"backend": backendSchema,

			"director": directorSchema,

			"force_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"cache_setting": cacheSettingSchema,

			"gzip": gzipSchema,

			"header": headerSchema,

			"s3logging": s3loggingSchema,

			"papertrail": papertrailSchema,

			"sumologic": sumologicSchema,

			"gcslogging": gcsloggingSchema,

			"bigquerylogging": bigqueryloggingSchema,

			"syslog": syslogSchema,

			"logentries": logentriesSchema,

			"splunk": splunkSchema,

			"blobstoragelogging": blobstoragelogging,

			"response_object": responseObjectSchema,

			"request_setting": requestSettingSchema,

			"vcl": vclSchema,

			"snippet": snippetSchema,
		},
	}
}

func resourceServiceV1Create(d *schema.ResourceData, meta interface{}) error {
	if err := validateVCLs(d); err != nil {
		return err
	}

	conn := meta.(*FastlyClient).conn
	service, err := conn.CreateService(&gofastly.CreateServiceInput{
		Name:    d.Get("name").(string),
		Comment: d.Get("comment").(string),
	})

	if err != nil {
		return err
	}

	d.SetId(service.ID)
	return resourceServiceV1Update(d, meta)
}

func resourceServiceV1Update(d *schema.ResourceData, meta interface{}) error {
	if err := validateVCLs(d); err != nil {
		return err
	}

	conn := meta.(*FastlyClient).conn

	// Update Name and/or Comment. No new verions is required for this
	if d.HasChange("name") || d.HasChange("comment") {
		_, err := conn.UpdateService(&gofastly.UpdateServiceInput{
			ID:      d.Id(),
			Name:    d.Get("name").(string),
			Comment: d.Get("comment").(string),
		})
		if err != nil {
			return err
		}
	}

	// Once activated, Versions are locked and become immutable. This is true for
	// versions that are no longer active. For Domains, Backends, DefaultHost and
	// DefaultTTL, a new Version must be created first, and updates posted to that
	// Version. Loop these attributes and determine if we need to create a new version first
	var needsChange bool
	for _, v := range []string{
		"domain",
		"backend",
		"default_host",
		"default_ttl",
		"director",
		"header",
		"gzip",
		"healthcheck",
		"s3logging",
		"papertrail",
		"gcslogging",
		"bigquerylogging",
		"syslog",
		"sumologic",
		"logentries",
		"splunk",
		"blobstoragelogging",
		"response_object",
		"condition",
		"request_setting",
		"cache_setting",
		"snippet",
		"vcl",
	} {
		if d.HasChange(v) {
			needsChange = true
		}
	}

	// Update the active version's comment. No new version is required for this
	if d.HasChange("version_comment") && !needsChange {
		latestVersion := d.Get("active_version").(int)
		if latestVersion == 0 {
			// If the service was just created, there is an empty Version 1 available
			// that is unlocked and can be updated
			latestVersion = 1
		}

		opts := gofastly.UpdateVersionInput{
			Service: d.Id(),
			Version: latestVersion,
			Comment: d.Get("version_comment").(string),
		}

		log.Printf("[DEBUG] Update Version opts: %#v", opts)
		_, err := conn.UpdateVersion(&opts)
		if err != nil {
			return err
		}
	}

	if needsChange {
		latestVersion := d.Get("active_version").(int)
		if latestVersion == 0 {
			// If the service was just created, there is an empty Version 1 available
			// that is unlocked and can be updated
			latestVersion = 1
		} else {
			// Clone the latest version, giving us an unlocked version we can modify
			log.Printf("[DEBUG] Creating clone of version (%d) for updates", latestVersion)
			newVersion, err := conn.CloneVersion(&gofastly.CloneVersionInput{
				Service: d.Id(),
				Version: latestVersion,
			})
			if err != nil {
				return err
			}

			// The new version number is named "Number", but it's actually a string
			latestVersion = newVersion.Number

			// New versions are not immediately found in the API, or are not
			// immediately mutable, so we need to sleep a few and let Fastly ready
			// itself. Typically, 7 seconds is enough
			log.Print("[DEBUG] Sleeping 7 seconds to allow Fastly Version to be available")
			time.Sleep(7 * time.Second)

			// Update the cloned version's comment
			if d.Get("version_comment").(string) != "" {
				opts := gofastly.UpdateVersionInput{
					Service: d.Id(),
					Version: latestVersion,
					Comment: d.Get("version_comment").(string),
				}

				log.Printf("[DEBUG] Update Version opts: %#v", opts)
				_, err := conn.UpdateVersion(&opts)
				if err != nil {
					return err
				}
			}
		}

		// update general settings
		if d.HasChange("default_host") || d.HasChange("default_ttl") {
			opts := gofastly.UpdateSettingsInput{
				Service: d.Id(),
				Version: latestVersion,
				// default_ttl has the same default value of 3600 that is provided by
				// the Fastly API, so it's safe to include here
				DefaultTTL: uint(d.Get("default_ttl").(int)),
			}

			if attr, ok := d.GetOk("default_host"); ok {
				opts.DefaultHost = attr.(string)
			}

			log.Printf("[DEBUG] Update Settings opts: %#v", opts)
			_, err := conn.UpdateSettings(&opts)
			if err != nil {
				return err
			}
		}

		// Conditions need to be updated first, as they can be referenced by other
		// configuration objects (Backends, Request Headers, etc)

		// Find difference in Conditions
		if d.HasChange("condition") {
			// Note: we don't utilize the PUT endpoint to update these objects, we simply
			// destroy any that have changed, and create new ones with the updated
			// values. This is how Terraform works with nested sub resources, we only
			// get the full diff not a partial set item diff. Because this is done
			// on a new version of the Fastly Service configuration, this is considered safe

			oc, nc := d.GetChange("condition")
			if oc == nil {
				oc = new(schema.Set)
			}
			if nc == nil {
				nc = new(schema.Set)
			}

			ocs := oc.(*schema.Set)
			ncs := nc.(*schema.Set)
			removeConditions := ocs.Difference(ncs).List()
			addConditions := ncs.Difference(ocs).List()

			// DELETE old Conditions
			for _, cRaw := range removeConditions {
				cf := cRaw.(map[string]interface{})
				opts := gofastly.DeleteConditionInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    cf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Conditions Removal opts: %#v", opts)
				err := conn.DeleteCondition(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new Conditions
			for _, cRaw := range addConditions {
				cf := cRaw.(map[string]interface{})
				opts := gofastly.CreateConditionInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    cf["name"].(string),
					Type:    cf["type"].(string),
					// need to trim leading/tailing spaces, incase the config has HEREDOC
					// formatting and contains a trailing new line
					Statement: strings.TrimSpace(cf["statement"].(string)),
					Priority:  cf["priority"].(int),
				}

				log.Printf("[DEBUG] Create Conditions Opts: %#v", opts)
				_, err := conn.CreateCondition(&opts)
				if err != nil {
					return err
				}
			}
		}

		// Find differences in domains
		if d.HasChange("domain") {
			od, nd := d.GetChange("domain")
			if od == nil {
				od = new(schema.Set)
			}
			if nd == nil {
				nd = new(schema.Set)
			}

			ods := od.(*schema.Set)
			nds := nd.(*schema.Set)

			remove := ods.Difference(nds).List()
			add := nds.Difference(ods).List()

			// Delete removed domains
			for _, dRaw := range remove {
				df := dRaw.(map[string]interface{})
				opts := gofastly.DeleteDomainInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Domain removal opts: %#v", opts)
				err := conn.DeleteDomain(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new Domains
			for _, dRaw := range add {
				df := dRaw.(map[string]interface{})
				opts := gofastly.CreateDomainInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				if v, ok := df["comment"]; ok {
					opts.Comment = v.(string)
				}

				log.Printf("[DEBUG] Fastly Domain Addition opts: %#v", opts)
				_, err := conn.CreateDomain(&opts)
				if err != nil {
					return err
				}
			}
		}

		// Healthchecks need to be updated BEFORE backends
		if d.HasChange("healthcheck") {
			oh, nh := d.GetChange("healthcheck")
			if oh == nil {
				oh = new(schema.Set)
			}
			if nh == nil {
				nh = new(schema.Set)
			}

			ohs := oh.(*schema.Set)
			nhs := nh.(*schema.Set)
			removeHealthCheck := ohs.Difference(nhs).List()
			addHealthCheck := nhs.Difference(ohs).List()

			// DELETE old healthcheck configurations
			for _, hRaw := range removeHealthCheck {
				hf := hRaw.(map[string]interface{})
				opts := gofastly.DeleteHealthCheckInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    hf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Healthcheck removal opts: %#v", opts)
				err := conn.DeleteHealthCheck(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Healthcheck
			for _, hRaw := range addHealthCheck {
				hf := hRaw.(map[string]interface{})

				opts := gofastly.CreateHealthCheckInput{
					Service:          d.Id(),
					Version:          latestVersion,
					Name:             hf["name"].(string),
					Host:             hf["host"].(string),
					Path:             hf["path"].(string),
					CheckInterval:    uint(hf["check_interval"].(int)),
					ExpectedResponse: uint(hf["expected_response"].(int)),
					HTTPVersion:      hf["http_version"].(string),
					Initial:          uint(hf["initial"].(int)),
					Method:           hf["method"].(string),
					Threshold:        uint(hf["threshold"].(int)),
					Timeout:          uint(hf["timeout"].(int)),
					Window:           uint(hf["window"].(int)),
				}

				log.Printf("[DEBUG] Create Healthcheck Opts: %#v", opts)
				_, err := conn.CreateHealthCheck(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in backends
		if d.HasChange("backend") {
			ob, nb := d.GetChange("backend")
			if ob == nil {
				ob = new(schema.Set)
			}
			if nb == nil {
				nb = new(schema.Set)
			}

			obs := ob.(*schema.Set)
			nbs := nb.(*schema.Set)
			removeBackends := obs.Difference(nbs).List()
			addBackends := nbs.Difference(obs).List()

			// DELETE old Backends
			for _, bRaw := range removeBackends {
				bf := bRaw.(map[string]interface{})
				opts := gofastly.DeleteBackendInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    bf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Backend removal opts: %#v", opts)
				err := conn.DeleteBackend(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// Find and post new Backends
			for _, dRaw := range addBackends {
				df := dRaw.(map[string]interface{})
				opts := gofastly.CreateBackendInput{
					Service:             d.Id(),
					Version:             latestVersion,
					Name:                df["name"].(string),
					Address:             df["address"].(string),
					OverrideHost:        df["override_host"].(string),
					AutoLoadbalance:     gofastly.CBool(df["auto_loadbalance"].(bool)),
					SSLCheckCert:        gofastly.CBool(df["ssl_check_cert"].(bool)),
					SSLHostname:         df["ssl_hostname"].(string),
					SSLCACert:           df["ssl_ca_cert"].(string),
					SSLCertHostname:     df["ssl_cert_hostname"].(string),
					SSLSNIHostname:      df["ssl_sni_hostname"].(string),
					UseSSL:              gofastly.CBool(df["use_ssl"].(bool)),
					SSLClientKey:        df["ssl_client_key"].(string),
					SSLClientCert:       df["ssl_client_cert"].(string),
					MaxTLSVersion:       df["max_tls_version"].(string),
					MinTLSVersion:       df["min_tls_version"].(string),
					SSLCiphers:          strings.Split(df["ssl_ciphers"].(string), ","),
					Shield:              df["shield"].(string),
					Port:                uint(df["port"].(int)),
					BetweenBytesTimeout: uint(df["between_bytes_timeout"].(int)),
					ConnectTimeout:      uint(df["connect_timeout"].(int)),
					ErrorThreshold:      uint(df["error_threshold"].(int)),
					FirstByteTimeout:    uint(df["first_byte_timeout"].(int)),
					MaxConn:             uint(df["max_conn"].(int)),
					Weight:              uint(df["weight"].(int)),
					RequestCondition:    df["request_condition"].(string),
					HealthCheck:         df["healthcheck"].(string),
				}

				log.Printf("[DEBUG] Create Backend Opts: %#v", opts)
				_, err := conn.CreateBackend(&opts)
				if err != nil {
					return err
				}
			}
		}

		if d.HasChange("director") {
			od, nd := d.GetChange("director")
			if od == nil {
				od = new(schema.Set)
			}
			if nd == nil {
				nd = new(schema.Set)
			}

			ods := od.(*schema.Set)
			nds := nd.(*schema.Set)

			removeDirector := ods.Difference(nds).List()
			addDirector := nds.Difference(ods).List()

			// DELETE old director configurations
			for _, dRaw := range removeDirector {
				df := dRaw.(map[string]interface{})
				opts := gofastly.DeleteDirectorInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				log.Printf("[DEBUG] Director Removal opts: %#v", opts)
				err := conn.DeleteDirector(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Director
			for _, dRaw := range addDirector {
				df := dRaw.(map[string]interface{})
				opts := gofastly.CreateDirectorInput{
					Service:  d.Id(),
					Version:  latestVersion,
					Name:     df["name"].(string),
					Comment:  df["comment"].(string),
					Shield:   df["shield"].(string),
					Capacity: uint(df["capacity"].(int)),
					Quorum:   uint(df["quorum"].(int)),
					Retries:  uint(df["retries"].(int)),
				}

				switch df["type"].(int) {
				case 1:
					opts.Type = gofastly.DirectorTypeRandom
				case 2:
					opts.Type = gofastly.DirectorTypeRoundRobin
				case 3:
					opts.Type = gofastly.DirectorTypeHash
				case 4:
					opts.Type = gofastly.DirectorTypeClient
				}

				log.Printf("[DEBUG] Director Create opts: %#v", opts)
				_, err := conn.CreateDirector(&opts)
				if err != nil {
					return err
				}

				if v, ok := df["backends"]; ok {
					if len(v.(*schema.Set).List()) > 0 {
						for _, b := range v.(*schema.Set).List() {
							opts := gofastly.CreateDirectorBackendInput{
								Service:  d.Id(),
								Version:  latestVersion,
								Director: df["name"].(string),
								Backend:  b.(string),
							}

							log.Printf("[DEBUG] Director Backend Create opts: %#v", opts)
							_, err := conn.CreateDirectorBackend(&opts)
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}

		if d.HasChange("header") {
			oh, nh := d.GetChange("header")
			if oh == nil {
				oh = new(schema.Set)
			}
			if nh == nil {
				nh = new(schema.Set)
			}

			ohs := oh.(*schema.Set)
			nhs := nh.(*schema.Set)

			remove := ohs.Difference(nhs).List()
			add := nhs.Difference(ohs).List()

			// Delete removed headers
			for _, dRaw := range remove {
				df := dRaw.(map[string]interface{})
				opts := gofastly.DeleteHeaderInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Header removal opts: %#v", opts)
				err := conn.DeleteHeader(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new Headers
			for _, dRaw := range add {
				opts, err := buildHeader(dRaw.(map[string]interface{}))
				if err != nil {
					log.Printf("[DEBUG] Error building Header: %s", err)
					return err
				}
				opts.Service = d.Id()
				opts.Version = latestVersion

				log.Printf("[DEBUG] Fastly Header Addition opts: %#v", opts)
				_, err = conn.CreateHeader(opts)
				if err != nil {
					return err
				}
			}
		}

		// Find differences in Gzips
		if d.HasChange("gzip") {
			og, ng := d.GetChange("gzip")
			if og == nil {
				og = new(schema.Set)
			}
			if ng == nil {
				ng = new(schema.Set)
			}

			ogs := og.(*schema.Set)
			ngs := ng.(*schema.Set)

			remove := ogs.Difference(ngs).List()
			add := ngs.Difference(ogs).List()

			// Delete removed gzip rules
			for _, dRaw := range remove {
				df := dRaw.(map[string]interface{})
				opts := gofastly.DeleteGzipInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Gzip removal opts: %#v", opts)
				err := conn.DeleteGzip(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new Gzips
			for _, dRaw := range add {
				df := dRaw.(map[string]interface{})
				opts := gofastly.CreateGzipInput{
					Service:        d.Id(),
					Version:        latestVersion,
					Name:           df["name"].(string),
					CacheCondition: df["cache_condition"].(string),
				}

				if v, ok := df["content_types"]; ok {
					if len(v.(*schema.Set).List()) > 0 {
						var cl []string
						for _, c := range v.(*schema.Set).List() {
							cl = append(cl, c.(string))
						}
						opts.ContentTypes = strings.Join(cl, " ")
					}
				}

				if v, ok := df["extensions"]; ok {
					if len(v.(*schema.Set).List()) > 0 {
						var el []string
						for _, e := range v.(*schema.Set).List() {
							el = append(el, e.(string))
						}
						opts.Extensions = strings.Join(el, " ")
					}
				}

				log.Printf("[DEBUG] Fastly Gzip Addition opts: %#v", opts)
				_, err := conn.CreateGzip(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in s3logging
		if d.HasChange("s3logging") {
			os, ns := d.GetChange("s3logging")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)
			removeS3Logging := oss.Difference(nss).List()
			addS3Logging := nss.Difference(oss).List()

			// DELETE old S3 Log configurations
			for _, sRaw := range removeS3Logging {
				sf := sRaw.(map[string]interface{})
				opts := gofastly.DeleteS3Input{
					Service: d.Id(),
					Version: latestVersion,
					Name:    sf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly S3 Logging removal opts: %#v", opts)
				err := conn.DeleteS3(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated S3 Logging
			for _, sRaw := range addS3Logging {
				sf := sRaw.(map[string]interface{})

				// Fastly API will not error if these are omitted, so we throw an error
				// if any of these are empty
				for _, sk := range []string{"s3_access_key", "s3_secret_key"} {
					if sf[sk].(string) == "" {
						return fmt.Errorf("[ERR] No %s found for S3 Log stream setup for Service (%s)", sk, d.Id())
					}
				}

				opts := gofastly.CreateS3Input{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              sf["name"].(string),
					BucketName:        sf["bucket_name"].(string),
					AccessKey:         sf["s3_access_key"].(string),
					SecretKey:         sf["s3_secret_key"].(string),
					Period:            uint(sf["period"].(int)),
					GzipLevel:         uint(sf["gzip_level"].(int)),
					Domain:            sf["domain"].(string),
					Path:              sf["path"].(string),
					Format:            sf["format"].(string),
					FormatVersion:     uint(sf["format_version"].(int)),
					TimestampFormat:   sf["timestamp_format"].(string),
					ResponseCondition: sf["response_condition"].(string),
					MessageType:       sf["message_type"].(string),
					Placement:         sf["placement"].(string),
				}

				redundancy := strings.ToLower(sf["redundancy"].(string))
				switch redundancy {
				case "standard":
					opts.Redundancy = gofastly.S3RedundancyStandard
				case "reduced_redundancy":
					opts.Redundancy = gofastly.S3RedundancyReduced
				}

				log.Printf("[DEBUG] Create S3 Logging Opts: %#v", opts)
				_, err := conn.CreateS3(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in Papertrail
		if d.HasChange("papertrail") {
			os, ns := d.GetChange("papertrail")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)
			removePapertrail := oss.Difference(nss).List()
			addPapertrail := nss.Difference(oss).List()

			// DELETE old papertrail configurations
			for _, pRaw := range removePapertrail {
				pf := pRaw.(map[string]interface{})
				opts := gofastly.DeletePapertrailInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    pf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Papertrail removal opts: %#v", opts)
				err := conn.DeletePapertrail(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Papertrail
			for _, pRaw := range addPapertrail {
				pf := pRaw.(map[string]interface{})

				opts := gofastly.CreatePapertrailInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              pf["name"].(string),
					Address:           pf["address"].(string),
					Port:              uint(pf["port"].(int)),
					Format:            pf["format"].(string),
					ResponseCondition: pf["response_condition"].(string),
					Placement:         pf["placement"].(string),
				}

				log.Printf("[DEBUG] Create Papertrail Opts: %#v", opts)
				_, err := conn.CreatePapertrail(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in Sumologic
		if d.HasChange("sumologic") {
			os, ns := d.GetChange("sumologic")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)
			removeSumologic := oss.Difference(nss).List()
			addSumologic := nss.Difference(oss).List()

			// DELETE old sumologic configurations
			for _, pRaw := range removeSumologic {
				sf := pRaw.(map[string]interface{})
				opts := gofastly.DeleteSumologicInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    sf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Sumologic removal opts: %#v", opts)
				err := conn.DeleteSumologic(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Sumologic
			for _, pRaw := range addSumologic {
				sf := pRaw.(map[string]interface{})
				opts := gofastly.CreateSumologicInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              sf["name"].(string),
					URL:               sf["url"].(string),
					Format:            sf["format"].(string),
					FormatVersion:     sf["format_version"].(int),
					ResponseCondition: sf["response_condition"].(string),
					MessageType:       sf["message_type"].(string),
					Placement:         sf["placement"].(string),
				}

				log.Printf("[DEBUG] Create Sumologic Opts: %#v", opts)
				_, err := conn.CreateSumologic(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in gcslogging
		if d.HasChange("gcslogging") {
			os, ns := d.GetChange("gcslogging")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)
			removeGcslogging := oss.Difference(nss).List()
			addGcslogging := nss.Difference(oss).List()

			// DELETE old gcslogging configurations
			for _, pRaw := range removeGcslogging {
				sf := pRaw.(map[string]interface{})
				opts := gofastly.DeleteGCSInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    sf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly gcslogging removal opts: %#v", opts)
				err := conn.DeleteGCS(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated gcslogging
			for _, pRaw := range addGcslogging {
				sf := pRaw.(map[string]interface{})
				opts := gofastly.CreateGCSInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              sf["name"].(string),
					User:              sf["email"].(string),
					Bucket:            sf["bucket_name"].(string),
					SecretKey:         sf["secret_key"].(string),
					Format:            sf["format"].(string),
					Path:              sf["path"].(string),
					Period:            uint(sf["period"].(int)),
					GzipLevel:         uint8(sf["gzip_level"].(int)),
					TimestampFormat:   sf["timestamp_format"].(string),
					MessageType:       sf["message_type"].(string),
					ResponseCondition: sf["response_condition"].(string),
					Placement:         sf["placement"].(string),
				}

				log.Printf("[DEBUG] Create GCS Opts: %#v", opts)
				_, err := conn.CreateGCS(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in bigquerylogging
		if d.HasChange("bigquerylogging") {
			os, ns := d.GetChange("bigquerylogging")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)
			removeBigquerylogging := oss.Difference(nss).List()
			addBigquerylogging := nss.Difference(oss).List()

			// DELETE old bigquerylogging configurations
			for _, pRaw := range removeBigquerylogging {
				sf := pRaw.(map[string]interface{})
				opts := gofastly.DeleteBigQueryInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    sf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly bigquerylogging removal opts: %#v", opts)
				err := conn.DeleteBigQuery(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated bigquerylogging
			for _, pRaw := range addBigquerylogging {
				sf := pRaw.(map[string]interface{})
				opts := gofastly.CreateBigQueryInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              sf["name"].(string),
					ProjectID:         sf["project_id"].(string),
					Dataset:           sf["dataset"].(string),
					Table:             sf["table"].(string),
					User:              sf["email"].(string),
					SecretKey:         sf["secret_key"].(string),
					ResponseCondition: sf["response_condition"].(string),
					Template:          sf["template"].(string),
					Placement:         sf["placement"].(string),
				}

				if sf["format"].(string) != "" {
					opts.Format = sf["format"].(string)
				}

				log.Printf("[DEBUG] Create bigquerylogging opts: %#v", opts)
				_, err := conn.CreateBigQuery(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in Syslog
		if d.HasChange("syslog") {
			os, ns := d.GetChange("syslog")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)
			removeSyslog := oss.Difference(nss).List()
			addSyslog := nss.Difference(oss).List()

			// DELETE old syslog configurations
			for _, pRaw := range removeSyslog {
				slf := pRaw.(map[string]interface{})
				opts := gofastly.DeleteSyslogInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    slf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Syslog removal opts: %#v", opts)
				err := conn.DeleteSyslog(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Syslog
			for _, pRaw := range addSyslog {
				slf := pRaw.(map[string]interface{})

				opts := gofastly.CreateSyslogInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              slf["name"].(string),
					Address:           slf["address"].(string),
					Port:              uint(slf["port"].(int)),
					Format:            slf["format"].(string),
					FormatVersion:     uint(slf["format_version"].(int)),
					Token:             slf["token"].(string),
					UseTLS:            gofastly.CBool(slf["use_tls"].(bool)),
					TLSHostname:       slf["tls_hostname"].(string),
					TLSCACert:         slf["tls_ca_cert"].(string),
					ResponseCondition: slf["response_condition"].(string),
					MessageType:       slf["message_type"].(string),
					Placement:         slf["placement"].(string),
				}

				log.Printf("[DEBUG] Create Syslog Opts: %#v", opts)
				_, err := conn.CreateSyslog(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in Logentries
		if d.HasChange("logentries") {
			os, ns := d.GetChange("logentries")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)
			removeLogentries := oss.Difference(nss).List()
			addLogentries := nss.Difference(oss).List()

			// DELETE old logentries configurations
			for _, pRaw := range removeLogentries {
				slf := pRaw.(map[string]interface{})
				opts := gofastly.DeleteLogentriesInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    slf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Logentries removal opts: %#v", opts)
				err := conn.DeleteLogentries(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Logentries
			for _, pRaw := range addLogentries {
				slf := pRaw.(map[string]interface{})

				opts := gofastly.CreateLogentriesInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              slf["name"].(string),
					Port:              uint(slf["port"].(int)),
					UseTLS:            gofastly.CBool(slf["use_tls"].(bool)),
					Token:             slf["token"].(string),
					Format:            slf["format"].(string),
					FormatVersion:     uint(slf["format_version"].(int)),
					ResponseCondition: slf["response_condition"].(string),
					Placement:         slf["placement"].(string),
				}

				log.Printf("[DEBUG] Create Logentries Opts: %#v", opts)
				_, err := conn.CreateLogentries(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in Splunk logging configurations
		if d.HasChange("splunk") {
			os, ns := d.GetChange("splunk")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			oss := os.(*schema.Set)
			nss := ns.(*schema.Set)

			remove := oss.Difference(nss).List()
			add := nss.Difference(oss).List()

			// DELETE old Splunk logging configurations
			for _, sRaw := range remove {
				sf := sRaw.(map[string]interface{})
				opts := gofastly.DeleteSplunkInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    sf["name"].(string),
				}

				log.Printf("[DEBUG] Splunk removal opts: %#v", opts)
				err := conn.DeleteSplunk(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Splunk configurations
			for _, sRaw := range add {
				sf := sRaw.(map[string]interface{})
				opts := gofastly.CreateSplunkInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              sf["name"].(string),
					URL:               sf["url"].(string),
					Format:            sf["format"].(string),
					FormatVersion:     uint(sf["format_version"].(int)),
					ResponseCondition: sf["response_condition"].(string),
					Placement:         sf["placement"].(string),
					Token:             sf["token"].(string),
				}

				log.Printf("[DEBUG] Splunk create opts: %#v", opts)
				_, err := conn.CreateSplunk(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in Blob Storage logging configurations
		if d.HasChange("blobstoragelogging") {
			obsl, nbsl := d.GetChange("blobstoragelogging")
			if obsl == nil {
				obsl = new(schema.Set)
			}
			if nbsl == nil {
				nbsl = new(schema.Set)
			}

			obsls := obsl.(*schema.Set)
			nbsls := nbsl.(*schema.Set)

			remove := obsls.Difference(nbsls).List()
			add := nbsls.Difference(obsls).List()

			// DELETE old Blob Storage logging configurations
			for _, bslRaw := range remove {
				bslf := bslRaw.(map[string]interface{})
				opts := gofastly.DeleteBlobStorageInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    bslf["name"].(string),
				}

				log.Printf("[DEBUG] Blob Storage logging removal opts: %#v", opts)
				err := conn.DeleteBlobStorage(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Blob Storage logging configurations
			for _, bslRaw := range add {
				bslf := bslRaw.(map[string]interface{})
				opts := gofastly.CreateBlobStorageInput{
					Service:           d.Id(),
					Version:           latestVersion,
					Name:              bslf["name"].(string),
					Path:              bslf["path"].(string),
					AccountName:       bslf["account_name"].(string),
					Container:         bslf["container"].(string),
					SASToken:          bslf["sas_token"].(string),
					Period:            uint(bslf["period"].(int)),
					TimestampFormat:   bslf["timestamp_format"].(string),
					GzipLevel:         uint(bslf["gzip_level"].(int)),
					PublicKey:         bslf["public_key"].(string),
					Format:            bslf["format"].(string),
					FormatVersion:     uint(bslf["format_version"].(int)),
					MessageType:       bslf["message_type"].(string),
					Placement:         bslf["placement"].(string),
					ResponseCondition: bslf["response_condition"].(string),
				}

				log.Printf("[DEBUG] Blob Storage logging create opts: %#v", opts)
				_, err := conn.CreateBlobStorage(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in Response Object
		if d.HasChange("response_object") {
			or, nr := d.GetChange("response_object")
			if or == nil {
				or = new(schema.Set)
			}
			if nr == nil {
				nr = new(schema.Set)
			}

			ors := or.(*schema.Set)
			nrs := nr.(*schema.Set)
			removeResponseObject := ors.Difference(nrs).List()
			addResponseObject := nrs.Difference(ors).List()

			// DELETE old response object configurations
			for _, rRaw := range removeResponseObject {
				rf := rRaw.(map[string]interface{})
				opts := gofastly.DeleteResponseObjectInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    rf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Response Object removal opts: %#v", opts)
				err := conn.DeleteResponseObject(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Response Object
			for _, rRaw := range addResponseObject {
				rf := rRaw.(map[string]interface{})

				opts := gofastly.CreateResponseObjectInput{
					Service:          d.Id(),
					Version:          latestVersion,
					Name:             rf["name"].(string),
					Status:           uint(rf["status"].(int)),
					Response:         rf["response"].(string),
					Content:          rf["content"].(string),
					ContentType:      rf["content_type"].(string),
					RequestCondition: rf["request_condition"].(string),
					CacheCondition:   rf["cache_condition"].(string),
				}

				log.Printf("[DEBUG] Create Response Object Opts: %#v", opts)
				_, err := conn.CreateResponseObject(&opts)
				if err != nil {
					return err
				}
			}
		}

		// find difference in request settings
		if d.HasChange("request_setting") {
			os, ns := d.GetChange("request_setting")
			if os == nil {
				os = new(schema.Set)
			}
			if ns == nil {
				ns = new(schema.Set)
			}

			ors := os.(*schema.Set)
			nrs := ns.(*schema.Set)
			removeRequestSettings := ors.Difference(nrs).List()
			addRequestSettings := nrs.Difference(ors).List()

			// DELETE old Request Settings configurations
			for _, sRaw := range removeRequestSettings {
				sf := sRaw.(map[string]interface{})
				opts := gofastly.DeleteRequestSettingInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    sf["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Request Setting removal opts: %#v", opts)
				err := conn.DeleteRequestSetting(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new/updated Request Setting
			for _, sRaw := range addRequestSettings {
				opts, err := buildRequestSetting(sRaw.(map[string]interface{}))
				if err != nil {
					log.Printf("[DEBUG] Error building Requset Setting: %s", err)
					return err
				}
				opts.Service = d.Id()
				opts.Version = latestVersion

				log.Printf("[DEBUG] Create Request Setting Opts: %#v", opts)
				_, err = conn.CreateRequestSetting(opts)
				if err != nil {
					return err
				}
			}
		}

		// Find differences in VCLs
		if d.HasChange("vcl") {
			// Note: as above with Gzip and S3 logging, we don't utilize the PUT
			// endpoint to update a VCL, we simply destroy it and create a new one.
			oldVCLVal, newVCLVal := d.GetChange("vcl")
			if oldVCLVal == nil {
				oldVCLVal = new(schema.Set)
			}
			if newVCLVal == nil {
				newVCLVal = new(schema.Set)
			}

			oldVCLSet := oldVCLVal.(*schema.Set)
			newVCLSet := newVCLVal.(*schema.Set)

			remove := oldVCLSet.Difference(newVCLSet).List()
			add := newVCLSet.Difference(oldVCLSet).List()

			// Delete removed VCL configurations
			for _, dRaw := range remove {
				df := dRaw.(map[string]interface{})
				opts := gofastly.DeleteVCLInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				log.Printf("[DEBUG] Fastly VCL Removal opts: %#v", opts)
				err := conn.DeleteVCL(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}
			// POST new VCL configurations
			for _, dRaw := range add {
				df := dRaw.(map[string]interface{})
				opts := gofastly.CreateVCLInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
					Content: df["content"].(string),
				}

				log.Printf("[DEBUG] Fastly VCL Addition opts: %#v", opts)
				_, err := conn.CreateVCL(&opts)
				if err != nil {
					return err
				}

				// if this new VCL is the main
				if df["main"].(bool) {
					opts := gofastly.ActivateVCLInput{
						Service: d.Id(),
						Version: latestVersion,
						Name:    df["name"].(string),
					}
					log.Printf("[DEBUG] Fastly VCL activation opts: %#v", opts)
					_, err := conn.ActivateVCL(&opts)
					if err != nil {
						return err
					}

				}
			}
		}

		// Find differences in VCL snippets
		if d.HasChange("snippet") {
			// Note: as above with Gzip and S3 logging, we don't utilize the PUT
			// endpoint to update a VCL snippet, we simply destroy it and create a new one.
			oldSnippetVal, newSnippetVal := d.GetChange("snippet")
			if oldSnippetVal == nil {
				oldSnippetVal = new(schema.Set)
			}
			if newSnippetVal == nil {
				newSnippetVal = new(schema.Set)
			}

			oldSnippetSet := oldSnippetVal.(*schema.Set)
			newSnippetSet := newSnippetVal.(*schema.Set)

			remove := oldSnippetSet.Difference(newSnippetSet).List()
			add := newSnippetSet.Difference(oldSnippetSet).List()

			// Delete removed VCL Snippet configurations
			for _, dRaw := range remove {
				df := dRaw.(map[string]interface{})
				opts := gofastly.DeleteSnippetInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				log.Printf("[DEBUG] Fastly VCL Snippet Removal opts: %#v", opts)
				err := conn.DeleteSnippet(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new VCL Snippet configurations
			for _, dRaw := range add {
				opts, err := buildSnippet(dRaw.(map[string]interface{}))
				if err != nil {
					log.Printf("[DEBUG] Error building VCL Snippet: %s", err)
					return err
				}
				opts.Service = d.Id()
				opts.Version = latestVersion

				log.Printf("[DEBUG] Fastly VCL Snippet Addition opts: %#v", opts)
				_, err = conn.CreateSnippet(opts)
				if err != nil {
					return err
				}
			}
		}

		// Find differences in Cache Settings
		if d.HasChange("cache_setting") {
			oc, nc := d.GetChange("cache_setting")
			if oc == nil {
				oc = new(schema.Set)
			}
			if nc == nil {
				nc = new(schema.Set)
			}

			ocs := oc.(*schema.Set)
			ncs := nc.(*schema.Set)

			remove := ocs.Difference(ncs).List()
			add := ncs.Difference(ocs).List()

			// Delete removed Cache Settings
			for _, dRaw := range remove {
				df := dRaw.(map[string]interface{})
				opts := gofastly.DeleteCacheSettingInput{
					Service: d.Id(),
					Version: latestVersion,
					Name:    df["name"].(string),
				}

				log.Printf("[DEBUG] Fastly Cache Settings removal opts: %#v", opts)
				err := conn.DeleteCacheSetting(&opts)
				if errRes, ok := err.(*gofastly.HTTPError); ok {
					if errRes.StatusCode != 404 {
						return err
					}
				} else if err != nil {
					return err
				}
			}

			// POST new Cache Settings
			for _, dRaw := range add {
				opts, err := buildCacheSetting(dRaw.(map[string]interface{}))
				if err != nil {
					log.Printf("[DEBUG] Error building Cache Setting: %s", err)
					return err
				}
				opts.Service = d.Id()
				opts.Version = latestVersion

				log.Printf("[DEBUG] Fastly Cache Settings Addition opts: %#v", opts)
				_, err = conn.CreateCacheSetting(opts)
				if err != nil {
					return err
				}
			}
		}

		// validate version
		log.Printf("[DEBUG] Validating Fastly Service (%s), Version (%v)", d.Id(), latestVersion)
		valid, msg, err := conn.ValidateVersion(&gofastly.ValidateVersionInput{
			Service: d.Id(),
			Version: latestVersion,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error checking validation: %s", err)
		}

		if !valid {
			return fmt.Errorf("[ERR] Invalid configuration for Fastly Service (%s): %s", d.Id(), msg)
		}

		shouldActivate := d.Get("activate").(bool)
		if shouldActivate {
			log.Printf("[DEBUG] Activating Fastly Service (%s), Version (%v)", d.Id(), latestVersion)
			_, err = conn.ActivateVersion(&gofastly.ActivateVersionInput{
				Service: d.Id(),
				Version: latestVersion,
			})
			if err != nil {
				return fmt.Errorf("[ERR] Error activating version (%d): %s", latestVersion, err)
			}

			// Only if the version is valid and activated do we set the active_version.
			// This prevents us from getting stuck in cloning an invalid version
			d.Set("active_version", latestVersion)
		} else {
			log.Printf("[INFO] Skipping activation of Fastly Service (%s), Version (%v)", d.Id(), latestVersion)
			log.Print("[INFO] The Terraform definition is explicitly specified to not activate the changes on Fastly")
			log.Printf("[INFO] Version (%v) has been pushed and validated", latestVersion)
			log.Printf("[INFO] Visit https://manage.fastly.com/configure/services/%s/versions/%v and activate it manually", d.Id(), latestVersion)
		}
	}

	return resourceServiceV1Read(d, meta)
}

func resourceServiceV1Read(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*FastlyClient).conn

	// Find the Service. Discard the service because we need the ServiceDetails,
	// not just a Service record
	_, err := findService(d.Id(), meta)
	if err != nil {
		switch err {
		case fastlyNoServiceFoundErr:
			log.Printf("[WARN] %s for ID (%s)", err, d.Id())
			d.SetId("")
			return nil
		default:
			return err
		}
	}

	s, err := conn.GetServiceDetails(&gofastly.GetServiceInput{
		ID: d.Id(),
	})

	if err != nil {
		return err
	}

	d.Set("name", s.Name)
	d.Set("comment", s.Comment)
	d.Set("version_comment", s.Version.Comment)
	d.Set("active_version", s.ActiveVersion.Number)

	// If CreateService succeeds, but initial updates to the Service fail, we'll
	// have an empty ActiveService version (no version is active, so we can't
	// query for information on it)
	if s.ActiveVersion.Number != 0 {
		settingsOpts := gofastly.GetSettingsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		}
		if settings, err := conn.GetSettings(&settingsOpts); err == nil {
			d.Set("default_host", settings.DefaultHost)
			d.Set("default_ttl", settings.DefaultTTL)
		} else {
			return fmt.Errorf("[ERR] Error looking up Version settings for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		// TODO: update go-fastly to support an ActiveVersion struct, which contains
		// domain and backend info in the response. Here we do 2 additional queries
		// to find out that info
		log.Printf("[DEBUG] Refreshing Domains for (%s)", d.Id())
		domainList, err := conn.ListDomains(&gofastly.ListDomainsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Domains for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		// Refresh Domains
		dl := flattenDomains(domainList)

		if err := d.Set("domain", dl); err != nil {
			log.Printf("[WARN] Error setting Domains for (%s): %s", d.Id(), err)
		}

		// Refresh Backends
		log.Printf("[DEBUG] Refreshing Backends for (%s)", d.Id())
		backendList, err := conn.ListBackends(&gofastly.ListBackendsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Backends for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		bl := flattenBackends(backendList)

		if err := d.Set("backend", bl); err != nil {
			log.Printf("[WARN] Error setting Backends for (%s): %s", d.Id(), err)
		}

		// refresh directors
		log.Printf("[DEBUG] Refreshing Directors for (%s)", d.Id())
		directorList, err := conn.ListDirectors(&gofastly.ListDirectorsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Directors for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		log.Printf("[DEBUG] Refreshing Director Backends for (%s)", d.Id())
		var directorBackendList []*gofastly.DirectorBackend

		for _, director := range directorList {
			for _, backend := range backendList {
				directorBackendGet, err := conn.GetDirectorBackend(&gofastly.GetDirectorBackendInput{
					Service:  d.Id(),
					Version:  s.ActiveVersion.Number,
					Director: director.Name,
					Backend:  backend.Name,
				})
				if err == nil {
					directorBackendList = append(directorBackendList, directorBackendGet)
				}
			}
		}

		dirl := flattenDirectors(directorList, directorBackendList)

		if err := d.Set("director", dirl); err != nil {
			log.Printf("[WARN] Error setting Directors for (%s): %s", d.Id(), err)
		}

		// refresh headers
		log.Printf("[DEBUG] Refreshing Headers for (%s)", d.Id())
		headerList, err := conn.ListHeaders(&gofastly.ListHeadersInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Headers for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		hl := flattenHeaders(headerList)

		if err := d.Set("header", hl); err != nil {
			log.Printf("[WARN] Error setting Headers for (%s): %s", d.Id(), err)
		}

		// refresh gzips
		log.Printf("[DEBUG] Refreshing Gzips for (%s)", d.Id())
		gzipsList, err := conn.ListGzips(&gofastly.ListGzipsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Gzips for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		gl := flattenGzips(gzipsList)

		if err := d.Set("gzip", gl); err != nil {
			log.Printf("[WARN] Error setting Gzips for (%s): %s", d.Id(), err)
		}

		// refresh Healthcheck
		log.Printf("[DEBUG] Refreshing Healthcheck for (%s)", d.Id())
		healthcheckList, err := conn.ListHealthChecks(&gofastly.ListHealthChecksInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Healthcheck for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		hcl := flattenHealthchecks(healthcheckList)

		if err := d.Set("healthcheck", hcl); err != nil {
			log.Printf("[WARN] Error setting Healthcheck for (%s): %s", d.Id(), err)
		}

		// refresh S3 Logging
		log.Printf("[DEBUG] Refreshing S3 Logging for (%s)", d.Id())
		s3List, err := conn.ListS3s(&gofastly.ListS3sInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up S3 Logging for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		sl := flattenS3s(s3List)

		if err := d.Set("s3logging", sl); err != nil {
			log.Printf("[WARN] Error setting S3 Logging for (%s): %s", d.Id(), err)
		}

		// refresh Papertrail Logging
		log.Printf("[DEBUG] Refreshing Papertrail for (%s)", d.Id())
		papertrailList, err := conn.ListPapertrails(&gofastly.ListPapertrailsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Papertrail for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		pl := flattenPapertrails(papertrailList)

		if err := d.Set("papertrail", pl); err != nil {
			log.Printf("[WARN] Error setting Papertrail for (%s): %s", d.Id(), err)
		}

		// refresh Sumologic Logging
		log.Printf("[DEBUG] Refreshing Sumologic for (%s)", d.Id())
		sumologicList, err := conn.ListSumologics(&gofastly.ListSumologicsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Sumologic for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		sul := flattenSumologics(sumologicList)
		if err := d.Set("sumologic", sul); err != nil {
			log.Printf("[WARN] Error setting Sumologic for (%s): %s", d.Id(), err)
		}

		// refresh GCS Logging
		log.Printf("[DEBUG] Refreshing GCS for (%s)", d.Id())
		GCSList, err := conn.ListGCSs(&gofastly.ListGCSsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up GCS for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		gcsl := flattenGCS(GCSList)
		if err := d.Set("gcslogging", gcsl); err != nil {
			log.Printf("[WARN] Error setting gcs for (%s): %s", d.Id(), err)
		}

		// refresh BigQuery Logging
		log.Printf("[DEBUG] Refreshing BigQuery for (%s)", d.Id())
		BQList, err := conn.ListBigQueries(&gofastly.ListBigQueriesInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up BigQuery logging for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		bql := flattenBigQuery(BQList)
		if err := d.Set("bigquerylogging", bql); err != nil {
			log.Printf("[WARN] Error setting bigquerylogging for (%s): %s", d.Id(), err)
		}

		// refresh Syslog Logging
		log.Printf("[DEBUG] Refreshing Syslog for (%s)", d.Id())
		syslogList, err := conn.ListSyslogs(&gofastly.ListSyslogsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Syslog for (%s), version (%d): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		sll := flattenSyslogs(syslogList)

		if err := d.Set("syslog", sll); err != nil {
			log.Printf("[WARN] Error setting Syslog for (%s): %s", d.Id(), err)
		}

		// refresh Logentries Logging
		log.Printf("[DEBUG] Refreshing Logentries for (%s)", d.Id())
		logentriesList, err := conn.ListLogentries(&gofastly.ListLogentriesInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Logentries for (%s), version (%d): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		lel := flattenLogentries(logentriesList)

		if err := d.Set("logentries", lel); err != nil {
			log.Printf("[WARN] Error setting Logentries for (%s): %s", d.Id(), err)
		}

		// refresh Splunk Logging
		log.Printf("[DEBUG] Refreshing Splunks for (%s)", d.Id())
		splunkList, err := conn.ListSplunks(&gofastly.ListSplunksInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Splunks for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		spl := flattenSplunks(splunkList)

		if err := d.Set("splunk", spl); err != nil {
			log.Printf("[WARN] Error setting Splunks for (%s): %s", d.Id(), err)
		}

		// refresh Blob Storage Logging
		log.Printf("[DEBUG] Refreshing Blob Storages for (%s)", d.Id())
		blobStorageList, err := conn.ListBlobStorages(&gofastly.ListBlobStoragesInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Blob Storages for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		bsl := flattenBlobStorages(blobStorageList)

		if err := d.Set("blobstoragelogging", bsl); err != nil {
			log.Printf("[WARN] Error setting Blob Storages for (%s): %s", d.Id(), err)
		}

		// refresh Response Objects
		log.Printf("[DEBUG] Refreshing Response Object for (%s)", d.Id())
		responseObjectList, err := conn.ListResponseObjects(&gofastly.ListResponseObjectsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Response Object for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		rol := flattenResponseObjects(responseObjectList)

		if err := d.Set("response_object", rol); err != nil {
			log.Printf("[WARN] Error setting Response Object for (%s): %s", d.Id(), err)
		}

		// refresh Conditions
		log.Printf("[DEBUG] Refreshing Conditions for (%s)", d.Id())
		conditionList, err := conn.ListConditions(&gofastly.ListConditionsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Conditions for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		cl := flattenConditions(conditionList)

		if err := d.Set("condition", cl); err != nil {
			log.Printf("[WARN] Error setting Conditions for (%s): %s", d.Id(), err)
		}

		// refresh Request Settings
		log.Printf("[DEBUG] Refreshing Request Settings for (%s)", d.Id())
		rsList, err := conn.ListRequestSettings(&gofastly.ListRequestSettingsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Request Settings for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		rl := flattenRequestSettings(rsList)

		if err := d.Set("request_setting", rl); err != nil {
			log.Printf("[WARN] Error setting Request Settings for (%s): %s", d.Id(), err)
		}

		// refresh VCLs
		log.Printf("[DEBUG] Refreshing VCLs for (%s)", d.Id())
		vclList, err := conn.ListVCLs(&gofastly.ListVCLsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})
		if err != nil {
			return fmt.Errorf("[ERR] Error looking up VCLs for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		vl := flattenVCLs(vclList)

		if err := d.Set("vcl", vl); err != nil {
			log.Printf("[WARN] Error setting VCLs for (%s): %s", d.Id(), err)
		}

		// refresh VCL Snippets
		log.Printf("[DEBUG] Refreshing VCL Snippets for (%s)", d.Id())
		snippetList, err := conn.ListSnippets(&gofastly.ListSnippetsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})
		if err != nil {
			return fmt.Errorf("[ERR] Error looking up VCL Snippets for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		vsl := flattenSnippets(snippetList)

		if err := d.Set("snippet", vsl); err != nil {
			log.Printf("[WARN] Error setting VCL Snippets for (%s): %s", d.Id(), err)
		}

		// refresh Cache Settings
		log.Printf("[DEBUG] Refreshing Cache Settings for (%s)", d.Id())
		cslList, err := conn.ListCacheSettings(&gofastly.ListCacheSettingsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})
		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Cache Settings for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		csl := flattenCacheSettings(cslList)

		if err := d.Set("cache_setting", csl); err != nil {
			log.Printf("[WARN] Error setting Cache Settings for (%s): %s", d.Id(), err)
		}

	} else {
		log.Printf("[DEBUG] Active Version for Service (%s) is empty, no state to refresh", d.Id())
	}

	return nil
}

func resourceServiceV1Delete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*FastlyClient).conn

	// Fastly will fail to delete any service with an Active Version.
	// If `force_destroy` is given, we deactivate the active version and then send
	// the DELETE call
	if d.Get("force_destroy").(bool) {
		s, err := conn.GetServiceDetails(&gofastly.GetServiceInput{
			ID: d.Id(),
		})

		if err != nil {
			return err
		}

		if s.ActiveVersion.Number != 0 {
			_, err := conn.DeactivateVersion(&gofastly.DeactivateVersionInput{
				Service: d.Id(),
				Version: s.ActiveVersion.Number,
			})
			if err != nil {
				return err
			}
		}
	}

	err := conn.DeleteService(&gofastly.DeleteServiceInput{
		ID: d.Id(),
	})

	if err != nil {
		return err
	}

	_, err = findService(d.Id(), meta)
	if err != nil {
		switch err {
		// we expect no records to be found here
		case fastlyNoServiceFoundErr:
			d.SetId("")
			return nil
		default:
			return err
		}
	}

	// findService above returned something and nil error, but shouldn't have
	return fmt.Errorf("[WARN] Tried deleting Service (%s), but was still found", d.Id())

}

// findService finds a Fastly Service via the ListServices endpoint, returning
// the Service if found.
//
// Fastly API does not include any "deleted_at" type parameter to indicate
// that a Service has been deleted. GET requests to a deleted Service will
// return 200 OK and have the full output of the Service for an unknown time
// (days, in my testing). In order to determine if a Service is deleted, we
// need to hit /service and loop the returned Services, searching for the one
// in question. This endpoint only returns active or "alive" services. If the
// Service is not included, then it's "gone"
//
// Returns a fastlyNoServiceFoundErr error if the Service is not found in the
// ListServices response.
func findService(id string, meta interface{}) (*gofastly.Service, error) {
	conn := meta.(*FastlyClient).conn

	l, err := conn.ListServices(&gofastly.ListServicesInput{})
	if err != nil {
		return nil, fmt.Errorf("[WARN] Error listing services (%s): %s", id, err)
	}

	for _, s := range l {
		if s.ID == id {
			log.Printf("[DEBUG] Found Service (%s)", id)
			return s, nil
		}
	}

	return nil, fastlyNoServiceFoundErr
}
