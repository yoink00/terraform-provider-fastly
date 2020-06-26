package fastly

import (
	"fmt"
	"reflect"
	"testing"

	gofastly "github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)


func TestResourceFastlyFlattenPackage(t *testing.T) {
	cases := []struct {
		remotePackage gofastly.Package
		remoteFilename string
		local  []map[string]interface{}
	}{
		{
			remotePackage: gofastly.Package{
					Metadata: gofastly.PackageMetadata{
						Size:    1234567,
						HashSum: "ouqwhgohqwoghoqhwrbowrboqwohbipwqbrwib",
				},
			},
			remoteFilename: "test_file.tar.gz",
			local: []map[string]interface{}{
				{
					"source_code_hash": "ouqwhgohqwoghoqhwrbowrboqwohbipwqbrwib",
					"source_code_size": 1234567,
					"filename":         "test_file.tar.gz",
				},
			},
		},
	}

	for _, c := range cases {
		out := flattenPackage(&c.remotePackage, c.remoteFilename)
		if !reflect.DeepEqual(out, c.local) {
			t.Fatalf("Error matching:\nexpected: %#v\ngot: %#v", c.local, out)
		}
	}
}



func TestAccFastlyServiceV1_package_basic(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domain := fmt.Sprintf("fastly-test.%s.com", name)

	wp1 := gofastly.Package{
		Metadata: gofastly.PackageMetadata{
			Name:        "wasm-test",
			Description: "Test Package",
			Authors:     []string{"fastly@fastly.com"},
			Language:    "rust",
			Size:        2015936,
			HashSum:     "f99485bd301e23f028474d26d398da525de17a372ae9e7026891d7f85361d2540d14b3b091929c3f170eade573595e20b3405a9e29651ede59915f2e1652f616",
		},
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1PackageConfig(name, domain),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_wasm_v1.foo", &service),
					testAccCheckFastlyServiceV1PackageAttributes(&service, &wp1),
					resource.TestCheckResourceAttr(
						"fastly_service_wasm_v1.foo", "name", name),
					resource.TestCheckResourceAttr(
						"fastly_service_wasm_v1.foo", "package.#", "1"),
				),
			},
		},
	})
}

func testAccCheckFastlyServiceV1PackageAttributes(service *gofastly.ServiceDetail, wasmPackage *gofastly.Package) resource.TestCheckFunc {
	return func(s *terraform.State) error {

		conn := testAccProvider.Meta().(*FastlyClient).conn
		wp, err := conn.GetPackage(&gofastly.GetPackageInput{
			Service: service.ID,
			Version: service.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Package for (%s), version (%d): %s", service.Name, service.ActiveVersion.Number, err)
		}

		if wasmPackage.Metadata.Size != wp.Metadata.Size {
			return fmt.Errorf("Package size mismatch, expected: %v, got: %v", wasmPackage.Metadata.Size, wp.Metadata.Size)
		}

		if wasmPackage.Metadata.HashSum != wp.Metadata.HashSum {
			return fmt.Errorf("Package hashsum mismatch, expected: %v, got: %v", wasmPackage.Metadata.HashSum, wp.Metadata.HashSum)
		}

		if wasmPackage.Metadata.Language != wp.Metadata.Language {
			return fmt.Errorf("Package language mismatch, expected: %v, got: %v", wasmPackage.Metadata.Language, wp.Metadata.Language)
		}

		if wasmPackage.Metadata.Name != wp.Metadata.Name {
			return fmt.Errorf("Package name mismatch, expected: %v, got: %v", wasmPackage.Metadata.Name, wp.Metadata.Name)
		}

		return nil
	}
}

func testAccServiceV1PackageConfig(name string, domain string) string {
	return fmt.Sprintf(`
resource "fastly_service_wasm_v1" "foo" {
  name = "%s"
  domain {
    name    = "%s"
    comment = "tf-package-test"
  }
  backend {
    address = "aws.amazon.com"
    name    = "amazon docs"
  }
  package {
    filename = "test_fixtures/package/valid.tar.gz"
	source_code_hash = filesha512("test_fixtures/package/valid.tar.gz")
  }
  force_destroy = true
}
`, name, domain)
}
