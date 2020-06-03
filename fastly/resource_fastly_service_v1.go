package fastly

import (
	"errors"
	"fmt"
	"log"
	"time"

	gofastly "github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
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

			// Cloned Version represents the latest cloned version by the provider. It
			// gets set whenever Terraform detects changes and clones the currently
			// activated version in order to modify it. Active Version and Cloned
			// Version can be different if the Activate field is set to false in order
			// to prevent the service from being activated. It is not used internally,
			// but it is exported for users to see after running `terraform apply`.
			"cloned_version": {
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
			"backend":     backendSchema,
			"director":    directorSchema,

			"force_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"cache_setting":      cacheSettingSchema,
			"gzip":               gzipSchema,
			"header":             headerSchema,
			"s3logging":          s3loggingSchema,
			"papertrail":         papertrailSchema,
			"sumologic":          sumologicSchema,
			"gcslogging":         gcsloogingSchema,
			"bigquerylogging":    bigqueryloggingSchema,
			"syslog":             syslogSchema,
			"logentries":         logentriesSchema,
			"splunk":             splunkSchema,
			"blobstoragelogging": blogstorageloggingSchema,
			"httpslogging":       httpsloggingSchema,
			"response_object":    responseObjectSchema,
			"request_setting":    requestSettingSchema,
			"vcl":                vclSchema,
			"snippet":            snippetSchema,
			"dynamicsnippet":     dynamicsnippetSchema,
			"acl":                aclSchema,
			"dictionary":         dictionarySchema,
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
		"httpslogging",
		"response_object",
		"condition",
		"request_setting",
		"cache_setting",
		"snippet",
		"dynamicsnippet",
		"vcl",
		"acl",
		"dictionary",
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

	initialVersion := false

	if needsChange {
		latestVersion := d.Get("active_version").(int)
		if latestVersion == 0 {
			initialVersion = true
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
			d.Set("cloned_version", latestVersion)

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

		// If the requested default_ttl is 0, and this is the first
		// version being created, HasChange will return false, but we need
		// to set it anyway, so ensure we update the settings in that
		// case.
		if d.HasChange("default_host") || d.HasChange("default_ttl") || (d.Get("default_ttl") == 0 && initialVersion) {
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
		// configuraiton objects (Backends, Request Headers, etc)

		// Find difference in Conditions
		if d.HasChange("condition") {
			err, done := processCondition(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in domains
		if d.HasChange("domain") {
			err, done := processDomain(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Healthchecks need to be updated BEFORE backends
		if d.HasChange("healthcheck") {
			err, done := processHealthcheck(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in backends
		if d.HasChange("backend") {
			err, done := processBackend(d, latestVersion, conn)
			if done {
				return err
			}
		}

		if d.HasChange("director") {
			err, done := processDirector(d, latestVersion, conn)
			if done {
				return err
			}
		}

		if d.HasChange("header") {
			err, done := processHeader(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in Gzips
		if d.HasChange("gzip") {
			err, done := procesGzip(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in s3logging
		if d.HasChange("s3logging") {
			err, done := processS3logging(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in Papertrail
		if d.HasChange("papertrail") {
			err, done := processPapertrail(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in Sumologic
		if d.HasChange("sumologic") {
			err, done := processSumologic(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in gcslogging
		if d.HasChange("gcslogging") {
			err, done := processGcslogging(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in bigquerylogging
		if d.HasChange("bigquerylogging") {
			err, done := processBigquerylogging(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in Syslog
		if d.HasChange("syslog") {
			err, done := procesSyslog(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in Logentries
		if d.HasChange("logentries") {
			err, done := processLogentries(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in Splunk logging configurations
		if d.HasChange("splunk") {
			err, done := processSplunk(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in Blob Storage logging configurations
		if d.HasChange("blobstoragelogging") {
			err, done := processBlobstoragelogging(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find differences in HTTPS logging configuration
		if d.HasChange("httpslogging") {
			if err := processHTTPS(d, conn, latestVersion); err != nil {
				return err
			}
		}

		// find difference in Response Object
		if d.HasChange("response_object") {
			err, done := processResponseObject(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// find difference in request settings
		if d.HasChange("request_setting") {
			err, done := processRequestSetting(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in VCLs
		if d.HasChange("vcl") {
			err, done := processVcl(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in VCL snippets
		if d.HasChange("snippet") {
			err, done := processSnippet(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in VCL dynamic snippets
		if d.HasChange("dynamicsnippet") {
			err, done := processDynamicsnippet(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in Cache Settings
		if d.HasChange("cache_setting") {
			err, done := processCacheSetting(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in ACLs
		if d.HasChange("acl") {

			err, done := processAcl(d, latestVersion, conn)
			if done {
				return err
			}
		}

		// Find differences in dictionary
		if d.HasChange("dictionary") {

			err, done := processDictionary(d, latestVersion, conn)
			if done {
				return err
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

		// Refresh HTTPS
		if err := readHTTPS(conn, d, s); err != nil {
			return err
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

		// refresh ACLs
		log.Printf("[DEBUG] Refreshing ACLs for (%s)", d.Id())
		aclList, err := conn.ListACLs(&gofastly.ListACLsInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})
		if err != nil {
			return fmt.Errorf("[ERR] Error looking up ACLs for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		al := flattenACLs(aclList)

		if err := d.Set("acl", al); err != nil {
			log.Printf("[WARN] Error setting ACLs for (%s): %s", d.Id(), err)
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

		dynamicSnippets := flattenDynamicSnippets(snippetList)

		if err := d.Set("dynamicsnippet", dynamicSnippets); err != nil {
			log.Printf("[WARN] Error setting VCL Dynamic Snippets for (%s): %s", d.Id(), err)
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

		// refresh Dictionaries
		log.Printf("[DEBUG] Refreshing Dictionaries for (%s)", d.Id())
		dictList, err := conn.ListDictionaries(&gofastly.ListDictionariesInput{
			Service: d.Id(),
			Version: s.ActiveVersion.Number,
		})
		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Dictionaries for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
		}

		dict := flattenDictionaries(dictList)

		if err := d.Set("dictionary", dict); err != nil {
			log.Printf("[WARN] Error setting Dictionary for (%s): %s", d.Id(), err)
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
