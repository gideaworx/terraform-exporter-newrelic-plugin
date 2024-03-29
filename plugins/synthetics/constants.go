package synthetics

const (
	tfSimpleMonitorType = "newrelic_synthetics_monitor"
	tfStepMonitorType   = "newrelic_synthetics_monitor_step"
	tfScriptMonitorType = "newrelic_synthetics_script_monitor"

	getMonitors = `query($query: String!) {
  actor {
    entitySearch(query: $query) {
      results {
        entities {
          ... on SyntheticMonitorEntityOutline {
            guid
            name
            monitorType
						monitoredUrl
						goldenTags {
              tags {
                key
              }
            }
            tags {
              key
              values
            }
          }
        }
      }
    }
  }
}
`
	getSteps = `query($accountID: Int!, $guid: EntityGuid!) {
  actor {
    account(id: $accountID) {
      synthetics {
        steps(monitorGuid: $guid) {
          ordinal
          type
          values
        }
      }
    }
  }
}
`

	getScript = `query($accountID: Int!, $guid: EntityGuid!) {
	actor {
	  account(id: $accountID) {
			synthetics {
				script(monitorGuid: $guid) {
					text
				}
			}
	  }
	}
}
`

	providerTF = `# Configure the New Relic provider
provider "newrelic" {
	account_id = var.account_id
	api_key    = var.api_key
	region     = "US"
}

variable "account_id" {
	type        = number
	description = "The New Relic Account ID"
	default     = %d
}

variable "api_key" {
	type        = string
	description = "The New Relic API Key"
}
`
)

// these should be treated as constants as well, but Go
// does not allow map or slice types to be constant
var (
	periodMap = map[string]string{
		"1":    "EVERY_MINUTE",
		"5":    "EVERY_5_MINUTES",
		"10":   "EVERY_10_MINUTES",
		"15":   "EVERY_15_MINUTES",
		"30":   "EVERY_30_MINUTES",
		"60":   "EVERY_HOUR",
		"360":  "EVERY_6_HOURS",
		"720":  "EVERY_12_HOURS",
		"1440": "EVERY_DAY",
	}

	regionMap = map[string]string{
		"Montreal, Québec, CA":   "CA_CENTRAL_1",
		"Washington, DC, USA":    "US_EAST_1",
		"Columbus, OH, USA":      "US_EAST_2",
		"San Francisco, CA, USA": "US_WEST_1",
		"Portland, OR, USA":      "US_WEST_2",
	}
)
