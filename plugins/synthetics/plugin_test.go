package synthetics_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/newrelic/newrelic-client-go/newrelic"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gideaworx/terraform-exporter-newrelic-plugin/internal"
	"github.com/gideaworx/terraform-exporter-newrelic-plugin/plugins/synthetics"
	plugin "github.com/gideaworx/terraform-exporter-plugin-go"
)

var _ = Describe("Plugin", func() {
	It("Generates help", func() {
		s := &synthetics.SyntheticExporterCommand{}
		helpText, err := s.Help()
		Expect(err).NotTo(HaveOccurred())
		Expect(helpText).To(Equal(`
Flags:
  -i, --account-id=INT          The New Relic Account ID
  -k, --api-key=STRING          An API Key for the New Relic Acccount ID
  -m, --monitor-id=MONITOR-ID,...
                                The individual synthetic monitor ID to export.
                                May be specified multiple times.
  -q, --locator-query=STRING    The query used with NerdGraph to find monitors
                                to export.
`))
	})

	It("Generates an info", func() {
		s := &synthetics.SyntheticExporterCommand{}
		info, err := s.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Version).To(Equal(plugin.FromString(synthetics.Version)))
	})

	Describe("Export", func() {
		var (
			server  *httptest.Server
			command *synthetics.SyntheticExporterCommand
		)

		BeforeEach(func() {
			server = mockNerdGraphServer()

		})

		AfterEach(func() {
			server.Close()
		})

		Describe("Creating Files", func() {
			var outputDirectory string
			var err error
			BeforeEach(func() {
				outputDirectory, err = os.MkdirTemp("", "nrtftmp")
				Expect(err).NotTo(HaveOccurred())

				command = synthetics.NewSyntheticExporterCommand(
					newrelic.ConfigBaseURL(server.URL),
					newrelic.ConfigNerdGraphBaseURL(server.URL),
				)
			})

			AfterEach(func() {
				err := os.RemoveAll(outputDirectory)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Generates all monitors' files in outputDirectory", func() {
				resp, err := command.Export(plugin.ExportCommandRequest{
					OutputDirectory:    outputDirectory,
					SkipProviderOutput: false,
					PluginArgs: []string{
						"-i", "56789",
						"-k", "1234",
						"-w", "1",
						"-q", "domain = 'SYNTH'",
						"-a",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				var responseJSON struct {
					Data synthetics.MonitorSearchResponse `json:"data"`
				}

				b, err := os.ReadFile("testdata/get_monitors.json")
				Expect(err).NotTo(HaveOccurred())

				err = json.Unmarshal(b, &responseJSON)
				Expect(err).NotTo(HaveOccurred())

				generatedFiles := 0
				filepath.WalkDir(outputDirectory, func(path string, d fs.DirEntry, err error) error {
					if strings.HasSuffix(path, ".tf") {
						generatedFiles++
					}

					return nil
				})

				// this checks for one terraform file for each monitor plus newrelic_provider.tf
				Expect(generatedFiles).To(Equal(len(responseJSON.Data.Actor.EntitySearch.Results.Entities) + 1))
				Expect(filepath.Join(outputDirectory, ".account_id")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDirectory, "newrelic_provider_56789.tf")).To(BeAnExistingFile())
				for _, entity := range responseJSON.Data.Actor.EntitySearch.Results.Entities {
					scName := internal.ToSnakeCase(entity.Name)
					Expect(filepath.Join(outputDirectory, scName+".tf")).To(BeAnExistingFile())
					foundDirective := false
					for _, directive := range resp.Directives {
						if directive.ID == entity.GUID {
							foundDirective = true
							break
						}
					}

					if !foundDirective {
						Fail(fmt.Sprintf("Monitor GUID %s has an output file but doesn't have an associated import directive", entity.GUID))
					}
				}
			})

			It("Generates specific monitors' files in outputDirectory", func() {
				_, err := command.Export(plugin.ExportCommandRequest{
					OutputDirectory:    outputDirectory,
					SkipProviderOutput: false,
					PluginArgs: []string{
						"-i", "56789",
						"-k", "1234",
						"-w", "1",
						"-m", "MTc4ODMzMHxTWU5USHxNT05JVE9SfDg0YmNkNWZhLWVhMzAtNDc5Yy04YmY0LTY3NzU2NTc1ZmQ1ZQ",
						"-m", "MTc4ODMzMHxTWU5USHxNT05JVE9SfGMxOWIyYWIzLWU0ZjktNDAxNC05NDgyLWZmNTkzYjZjM2RmOA",
						"-m", "MTc4ODMzMHxTWU5USHxNT05JVE9SfGY5ZjIwMzY5LTEwMzMtNDdmMy05ODBhLTY3ZGVkNTcxOWYxYQ",
						"-a",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				generatedFiles := 0
				filepath.WalkDir(outputDirectory, func(path string, d fs.DirEntry, err error) error {
					if strings.HasSuffix(path, ".tf") || filepath.Base(path) == ".account_id" {
						generatedFiles++
					}

					return nil
				})
				Expect(generatedFiles).To(Equal(5))
				Expect(filepath.Join(outputDirectory, ".account_id")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDirectory, "newrelic_provider_56789.tf")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDirectory, "monitor_name_2.tf")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDirectory, "monitor_name_9.tf")).To(BeAnExistingFile())
				Expect(filepath.Join(outputDirectory, "monitor_name_25.tf")).To(BeAnExistingFile())
			})
		})
	})
})

type nerdgraphQuery struct {
	Query     string `json:"query"`
	Variables struct {
		GUID string `json:"guid"`
	} `json:"variables"`
}

func mockNerdGraphServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			return
		}

		defer r.Body.Close()

		b, _ := io.ReadAll(r.Body)
		var request nerdgraphQuery
		if err := json.Unmarshal(b, &request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if strings.Contains(request.Query, "entitySearch") {
			data, err := os.ReadFile("testdata/get_monitors.json")
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintln(w, err.Error())
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		if strings.Contains(request.Query, "steps") {
			data, err := os.ReadFile(fmt.Sprintf("testdata/steps/%s.json", request.Variables.GUID))
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintln(w, err.Error())
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		if strings.Contains(request.Query, "script") {
			data, err := os.ReadFile(fmt.Sprintf("testdata/script/%s.json", request.Variables.GUID))
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintln(w, err.Error())
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte{})
	}))
}
