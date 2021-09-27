package tower

import (
	"testing"

	"github.com/Jeffail/gabs/v2"
)

func TestAddSpecsMap(t *testing.T) {
	// input JSON with 4 specs
	details, err := gabs.ParseJSON([]byte(`{
      "description": "",
      "name": "",
      "spec": [
        {
          "choices": "",
          "default": "",
          "max": null,
          "min": null,
          "question_description": "UnifiedOS disk image name. Currently supported are Red Hat Enterprise Linux 7, Windows Server 2016 and Windows Server 2019.",
          "question_name": "Disk image",
          "required": true,
          "type": "text",
          "variable": "unifiedos_image"
        },
        {
          "choices": "",
          "default": "s3.large.2",
          "max": null,
          "min": null,
          "question_description": "Defaults to 4 CPU/8GB memory. For more information see https://confluence.sbb.ch/display/OTC/Virtual+Machine+sizes",
          "question_name": "Instance type",
          "required": true,
          "type": "text",
          "variable": "provision_otc_instance_type"
        },
        {
          "choices": "",
          "default": 10,
          "max": 20000,
          "min": 10,
          "new_question": true,
          "question_description": "Disk size for persistent data (e.g. /var/data on Linux or D:\\ on Windows)",
          "question_name": "Persistent data size (in GB)",
          "required": true,
          "type": "integer",
          "variable": "unifiedos_data_disk_size"
        },
        {
          "choices": "",
          "default": 10,
          "max": 500,
          "min": 10,
          "question_description": "Disk size for operating system (Linux: min 10GB / Windows: min 60GB)",
          "question_name": "Root disk size (in GB)",
          "required": true,
          "type": "integer",
          "variable": "unifiedos_root_disk_size"
        }
      ]
    }`))
	if err != nil {
		t.Error("Invalid JSON!")
		return
	}
	// expected output JSON with the map (see "specsMap" field)
	detailsWithMap, err := gabs.ParseJSON([]byte(`{
      "description": "",
      "name": "",
      "spec": [
        {
          "choices": "",
          "default": "",
          "max": null,
          "min": null,
          "question_description": "UnifiedOS disk image name. Currently supported are Red Hat Enterprise Linux 7, Windows Server 2016 and Windows Server 2019.",
          "question_name": "Disk image",
          "required": true,
          "type": "text",
          "variable": "unifiedos_image"
        },
        {
          "choices": "",
          "default": "s3.large.2",
          "max": null,
          "min": null,
          "question_description": "Defaults to 4 CPU/8GB memory. For more information see https://confluence.sbb.ch/display/OTC/Virtual+Machine+sizes",
          "question_name": "Instance type",
          "required": true,
          "type": "text",
          "variable": "provision_otc_instance_type"
        },
        {
          "choices": "",
          "default": 10,
          "max": 20000,
          "min": 10,
          "new_question": true,
          "question_description": "Disk size for persistent data (e.g. /var/data on Linux or D:\\ on Windows)",
          "question_name": "Persistent data size (in GB)",
          "required": true,
          "type": "integer",
          "variable": "unifiedos_data_disk_size"
        },
        {
          "choices": "",
          "default": 10,
          "max": 500,
          "min": 10,
          "question_description": "Disk size for operating system (Linux: min 10GB / Windows: min 60GB)",
          "question_name": "Root disk size (in GB)",
          "required": true,
          "type": "integer",
          "variable": "unifiedos_root_disk_size"
        }
      ],
      "specsMap": {
        "unifiedos_image": {
		  "index": 0,
          "choices": "",
          "default": "",
          "max": null,
          "min": null,
          "question_description": "UnifiedOS disk image name. Currently supported are Red Hat Enterprise Linux 7, Windows Server 2016 and Windows Server 2019.",
          "question_name": "Disk image",
          "required": true,
          "type": "text",
          "variable": "unifiedos_image"
        },
        "provision_otc_instance_type": {
		  "index": 1,
		  "choices": "",
          "default": "s3.large.2",
          "max": null,
          "min": null,
          "question_description": "Defaults to 4 CPU/8GB memory. For more information see https://confluence.sbb.ch/display/OTC/Virtual+Machine+sizes",
          "question_name": "Instance type",
          "required": true,
          "type": "text",
          "variable": "provision_otc_instance_type"
        },
        "unifiedos_data_disk_size": {
		  "index": 2,
          "choices": "",
          "default": 10,
          "max": 20000,
          "min": 10,
          "new_question": true,
          "question_description": "Disk size for persistent data (e.g. /var/data on Linux or D:\\ on Windows)",
          "question_name": "Persistent data size (in GB)",
          "required": true,
          "type": "integer",
          "variable": "unifiedos_data_disk_size"
        },
        "unifiedos_root_disk_size": {
		  "index": 3,
          "choices": "",
          "default": 10,
          "max": 500,
          "min": 10,
          "question_description": "Disk size for operating system (Linux: min 10GB / Windows: min 60GB)",
          "question_name": "Root disk size (in GB)",
          "required": true,
          "type": "integer",
          "variable": "unifiedos_root_disk_size"
        }
	  }
    }`))
	if err != nil {
		t.Error("Invalid JSON!")
		return
	}
	err = addSpecsMap(details)
	if err != nil {
		t.Error("ERROR! function \"addSpecsMap\" should not throw error! JSON input is correct")
	}
	if details.String() != detailsWithMap.String() {
		t.Error("ERROR! detailsWithMap is not as expected...")
	}
}
