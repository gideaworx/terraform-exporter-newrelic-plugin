package synthetics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/kong"
	"github.com/gideaworx/terraform-exporter-newrelic-plugin/internal"
	plugin "github.com/gideaworx/terraform-exporter-plugin-go"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/newrelic/newrelic-client-go/newrelic"
	"github.com/zclconf/go-cty/cty"
)

var Version string = "0.0.1"

type SyntheticExporterCommand struct {
	AccountID           int      `short:"i" required:"true" help:"The New Relic Account ID"`
	APIKey              string   `short:"k" required:"true" help:"An API Key for the New Relic Acccount ID"`
	MonitorID           []string `short:"m" required:"true" xor:"locator" help:"The individual synthetic monitor ID to export. May be specified multiple times."`
	LocatorQuery        string   `short:"q" required:"true" xor:"locator" help:"The query used with NerdGraph to find monitors to export."`
	ParallelWorkers     uint     `short:"w" required:"true" default:"10" hidden:"true" help:"Number of monitors to export in parallel. Defaults to 10"`
	CreateAccountIdFile bool     `short:"a" hidden:"true" default:"true"`
	importCommands      []plugin.ImportDirective
	nrClient            *newrelic.NewRelic
	outputDirectory     string
	nrClientOptions     []newrelic.ConfigOption
	logger              hclog.Logger
}

func NewSyntheticExporterCommand(options ...newrelic.ConfigOption) *SyntheticExporterCommand {
	return &SyntheticExporterCommand{
		importCommands:  []plugin.ImportDirective{},
		nrClientOptions: options,
		logger: hclog.New(&hclog.LoggerOptions{
			Level:  hclog.Info,
			Output: os.Stderr,
		}),
	}
}

var errSkip = errors.New("don't export")

func (s *SyntheticExporterCommand) Help() (string, error) {
	return internal.PluginCommandHelp(s)
}

func (s *SyntheticExporterCommand) Info() (plugin.CommandInfo, error) {
	return plugin.CommandInfo{
		Name:        "newrelic-synthetic-monitors",
		Description: "Export New Relic Synthetic Monitors from the specified New Relic Account",
		Summary:     "Export New Relic Synthetic Monitors from the specified New Relic Account",
		Version:     plugin.FromString(Version),
	}, nil
}

func (s *SyntheticExporterCommand) Export(request plugin.ExportCommandRequest) (plugin.ExportResponse, error) {
	var k *kong.Kong
	k, err := kong.New(s)
	if err != nil {
		return plugin.ExportResponse{}, err
	}

	_, err = k.Parse(request.PluginArgs)
	if err != nil {
		return plugin.ExportResponse{}, err
	}

	// There isn't an easy way to query nerdgraph for individual monitors with the data we need, so
	// if we're asking for individual monitors, get them all and we'll filter them after the query
	if len(s.MonitorID) > 0 {
		s.LocatorQuery = "domain = 'SYNTH'"
	}

	ctx := context.Background()
	queryVariables := map[string]any{"query": s.LocatorQuery}
	s.nrClient, err = newrelic.New(append([]newrelic.ConfigOption{newrelic.ConfigPersonalAPIKey(s.APIKey)}, s.nrClientOptions...)...)
	if err != nil {
		return plugin.ExportResponse{}, err
	}
	s.outputDirectory = request.OutputDirectory

	// This collects all synthetics from NerdGraph, and parses the response into a GetMonitorsResponse instance.
	var response MonitorSearchResponse
	if err := s.nrClient.NerdGraph.QueryWithResponseAndContext(ctx, getMonitors, queryVariables, &response); err != nil {
		return plugin.ExportResponse{}, fmt.Errorf("error querying NerdGraph: %w", err)
	}

	if !request.SkipProviderOutput {
		// Create the provider declaration. Since this is static for our purposes, we can copy directly from
		// a string constant
		provider, err := os.Create(filepath.Join(request.OutputDirectory, fmt.Sprintf("newrelic_provider_%d.tf", s.AccountID)))
		if err != nil {
			return plugin.ExportResponse{}, fmt.Errorf("error creating provider file: %w", err)
		}

		fmt.Fprintf(provider, providerTF, s.AccountID)
		provider.Close()
	}

	if s.CreateAccountIdFile {
		accountID, err := os.Create(filepath.Join(request.OutputDirectory, ".account_id"))
		if err != nil {
			return plugin.ExportResponse{}, fmt.Errorf("could not create .account_id: %w", err)
		}
		defer accountID.Close()
		fmt.Fprint(accountID, s.AccountID)
	}

	entities := make([]MonitorEntity, 0, len(response.Actor.EntitySearch.Results.Entities))
	// if we're asking for individual entities, collect just them
	if len(s.MonitorID) > 0 {
		for _, e := range response.Actor.EntitySearch.Results.Entities {
			if internal.IndexOf(e.GUID, s.MonitorID) >= 0 {
				entities = append(entities, e)
				if len(entities) == len(s.MonitorID) {
					break
				}
			}
		}
	} else {
		entities = append(entities, response.Actor.EntitySearch.Results.Entities...)
	}

	queueSize := len(entities)
	errorCollector := make(chan error, queueSize)

	// goroutines cannot return values. To collect errors that could happen in a goroutine,
	// we create a channel that accepts delivers, then start a goroutine that watches that
	// channel for errors created. When an error is collected, add it to a single error
	// collecting all error messages, and at the end, return the error that was created if
	// any errors were sent.
	var commandError error = nil
	go func() {
		// ranging over a chan will return a value (in this case, an error) when one is
		// available on the chan, and will end when the chan is closed.
		for err := range errorCollector {
			if commandError == nil {
				commandError = errors.New("the following errors occurred exporting monitors")
			}

			commandError = fmt.Errorf("%s\n%s", commandError.Error(), err.Error())
		}
	}()

	// WaitGroups are used to synchronize on many concurrent units of work. We add an initial
	// size to the group, then each unit of work will call wg.Done(), which decrements the number
	// of units remaining. wg.Wait() will block until all the wg.Done() calls occur.
	wg := new(sync.WaitGroup)
	wg.Add(queueSize)

	// Create a chan that can accept all of our monitors at once, to ensure that it does not empty
	// before everything is sent, however unlikely.
	workQueue := make(chan MonitorEntity, queueSize)

	// Create a pool of goroutines set to the size of s.ParallelWorkers (10 by default) that can collect
	// work items. Each routine takes the context, the wait group (so it can call Done()), the queue of
	// work items, and the error collector chan (so it can send errors if one occurs)
	for i := 0; i < int(s.ParallelWorkers); i++ {
		go s.exportMonitor(wg, ctx, workQueue, errorCollector)
	}

	// Add all the monitors to the work queue, so the individual goroutines can pull off the monitors
	// as they're able to do so
	for _, monitor := range entities {
		workQueue <- monitor
	}

	wg.Wait()
	// we have to close the work queue first or wg.Wait() blocks forever
	close(workQueue)

	// if commandError is not nil, wrap it in an error the CLI understands to process all the successful
	// commands
	if commandError != nil {
		commandError = fmt.Errorf("%w: %v", plugin.ErrSomeExportsFailed, commandError)
	}

	// now that all of the work is done, we can close the error collector
	close(errorCollector)
	return plugin.ExportResponse{
		Directives: s.importCommands,
	}, commandError
}

// exportMonitor takes a monitor off a queue of monitors and exports it to file. it returns either an error, representing
// the command line args that would be sent to "terraform import". this is always executed inside a goroutine
func (s *SyntheticExporterCommand) exportMonitor(wg *sync.WaitGroup, ctx context.Context, work chan MonitorEntity, errors chan error) {
	// grab a monitor off the work queue, and pass that monitor to exportSingleMonitor to do the work, because
	// we can ensure that wg.Done is called via a defer, which protects against accidentally forgetting
	// calling it in a code branch
	for monitor := range work {
		if err := s.exportSingleMonitor(wg, ctx, monitor); err != nil {
			errors <- err
		}
	}
}

// exportSingleMonitor will choose the appropriate render method for the given monitor and call it, returning
// an error if one occurred (or if the monitor's type is unsupported), or adding to the list of import commands
func (s *SyntheticExporterCommand) exportSingleMonitor(wg *sync.WaitGroup, ctx context.Context, monitor MonitorEntity) error {
	defer wg.Done()

	s.logger.Info("Exporting Monitor %s", monitor.Name)

	var render func(context.Context, MonitorEntity) (plugin.ImportDirective, error)

	// at this time, SCRIPT_API monitors are not supported
	switch monitor.MonitorType {
	case "SIMPLE", "BROWSER":
		render = s.renderSimpleMonitor
	case "STEP_MONITOR":
		render = s.renderStepMonitor
	case "SCRIPT_BROWSER":
		render = s.renderScriptMonitor
	default:
		s.logger.Info("WARN:", hclog.Fmt("unsupported monitor type %q", monitor.MonitorType))
		return nil
	}

	importCmd, err := render(ctx, monitor)
	if err != nil {
		if errors.Is(err, errSkip) {
			// ignore this monitor
			return nil
		}

		s.logger.Info("ERR:", hclog.Fmt("error rendering monitor %q: %v", monitor.Name, err))
		return nil
	}

	s.importCommands = append(s.importCommands, importCmd)
	return nil
}

func (s *SyntheticExporterCommand) renderCommon(resourceType string, resourceName string, monitor MonitorEntity) *hclwrite.File {
	file := hclwrite.NewEmptyFile()
	block := file.Body().AppendNewBlock("resource", []string{resourceType, resourceName})

	block.Body().SetAttributeValue("name", cty.StringVal(monitor.Name))
	block.Body().SetAttributeValue("type", cty.StringVal(monitor.MonitorType))

	locations := []cty.Value{}
	period := "EVERY_MINUTE"
	status := "ENABLED"
	runtimeType := ""
	runtimeTypeVersion := ""
	tagBlocks := []*hclwrite.Block{}
	for _, tag := range monitor.Tags {
		if tag.Key == "publicLocation" {
			for _, val := range tag.Values {
				locations = append(locations, cty.StringVal(regionMap[val]))
			}
		}

		if tag.Key == "period" {
			period = periodMap[tag.Values[0]]
		}

		if tag.Key == "monitorStatus" {
			status = strings.ToUpper(tag.Values[0])
		}

		if tag.Key == "runtimeType" {
			runtimeType = tag.Values[0]
		}

		if tag.Key == "runtimeTypeVersion" {
			runtimeTypeVersion = tag.Values[0]
		}

		if internal.IndexOfWithField(tag, monitor.GoldenTags.Tags, "Key") < 0 &&
			len(tag.Values) > 0 {
			tagBlock := hclwrite.NewBlock("tag", nil)
			tagBlock.Body().SetAttributeValue("key", cty.StringVal(tag.Key))
			tagBlock.Body().SetAttributeValue("values", internal.ToCtyList(tag.Values))
			tagBlocks = append(tagBlocks, tagBlock)

		}
	}

	if len(locations) == 0 {
		return nil
	}
	locationList := cty.ListVal(locations)
	block.Body().SetAttributeValue("locations_public", locationList)

	block.Body().AppendNewline()
	block.Body().SetAttributeValue("period", cty.StringVal(period))
	block.Body().SetAttributeValue("status", cty.StringVal(status))

	if runtimeType != "" {
		block.Body().SetAttributeValue("runtime_type", cty.StringVal(runtimeType))
	}

	if runtimeTypeVersion != "" {
		block.Body().SetAttributeValue("runtime_type_version", cty.StringVal(runtimeTypeVersion))
	}
	block.Body().AppendNewline()
	for _, b := range tagBlocks {
		block.Body().AppendBlock(b)
	}
	block.Body().AppendNewline()

	return file
}

func (s *SyntheticExporterCommand) renderSimpleMonitor(_ context.Context, monitor MonitorEntity) (plugin.ImportDirective, error) {
	tfResourceType := tfSimpleMonitorType
	tfResourceName := internal.ToSnakeCase(monitor.Name)

	file := s.renderCommon(tfResourceType, tfResourceName, monitor)
	if file == nil {
		return plugin.ImportDirective{}, errSkip
	}
	resourceBlock := file.Body().FirstMatchingBlock("resource", []string{tfResourceType, tfResourceName})
	resourceBlock.Body().SetAttributeValue("enable_screenshot_on_failure_and_script", cty.BoolVal(true))
	resourceBlock.Body().SetAttributeValue("bypass_head_request", cty.BoolVal(true))
	resourceBlock.Body().SetAttributeValue("verify_ssl", cty.BoolVal(true))
	resourceBlock.Body().SetAttributeValue("uri", cty.StringVal(monitor.MonitoredURL))

	for _, tag := range monitor.Tags {
		if tag.Key == "responseValidationText" {
			resourceBlock.Body().SetAttributeValue("validation_string", cty.StringVal(tag.Values[0]))
			break
		}
	}

	return s.printFile(file, monitor.GUID, tfResourceType, tfResourceName)
}

func (s *SyntheticExporterCommand) renderStepMonitor(ctx context.Context, monitor MonitorEntity) (plugin.ImportDirective, error) {
	tfResourceType := tfStepMonitorType
	tfResourceName := internal.ToSnakeCase(monitor.Name)

	file := s.renderCommon(tfResourceType, tfResourceName, monitor)
	if file == nil {
		return plugin.ImportDirective{}, errSkip
	}
	resourceBlock := file.Body().FirstMatchingBlock("resource", []string{tfResourceType, tfResourceName})

	vars := map[string]any{"accountID": s.AccountID, "guid": monitor.GUID}
	var response GetStepsResponse
	if err := s.nrClient.NerdGraph.QueryWithResponseAndContext(ctx, getSteps, vars, &response); err != nil {
		return plugin.ImportDirective{}, err
	}

	for _, step := range response.Actor.Account.Synthetics.Steps {
		block := resourceBlock.Body().AppendNewBlock("step", nil)
		block.Body().SetAttributeValue("ordinal", cty.NumberIntVal(step.Ordinal))
		block.Body().SetAttributeValue("type", cty.StringVal(step.Type))
		block.Body().SetAttributeValue("values", internal.ToCtyList(step.Values))
	}

	return s.printFile(file, monitor.GUID, tfResourceType, tfResourceName)
}

func (s *SyntheticExporterCommand) renderScriptMonitor(ctx context.Context, monitor MonitorEntity) (plugin.ImportDirective, error) {
	tfResourceType := tfScriptMonitorType
	tfResourceName := internal.ToSnakeCase(monitor.Name)

	file := s.renderCommon(tfResourceType, tfResourceName, monitor)
	if file == nil {
		return plugin.ImportDirective{}, errSkip
	}
	resourceBlock := file.Body().FirstMatchingBlock("resource", []string{tfResourceType, tfResourceName})

	vars := map[string]any{"accountID": s.AccountID, "guid": monitor.GUID}
	var response GetScriptResponse
	if err := s.nrClient.NerdGraph.QueryWithResponseAndContext(ctx, getScript, vars, &response); err != nil {
		return plugin.ImportDirective{}, err
	}

	for _, tag := range monitor.Tags {
		if tag.Key == "scriptLanguage" {
			resourceBlock.Body().SetAttributeValue("script_language", cty.StringVal(tag.Values[0]))
		}
	}

	resourceBlock.Body().SetAttributeRaw("script", internal.CreateHeredoc(response.Actor.Account.Synthetics.Script.Text, "-SCRIPT", true))

	return s.printFile(file, monitor.GUID, tfResourceType, tfResourceName)
}

func (s *SyntheticExporterCommand) printFile(file *hclwrite.File, monitorGUID string, tfResourceType string, tfResourceName string) (plugin.ImportDirective, error) {
	path, err := filepath.Abs(filepath.Join(s.outputDirectory, tfResourceName))
	if err != nil {
		return plugin.ImportDirective{}, err
	}

	filePtr, err := os.Create(fmt.Sprintf("%s.tf", path))
	if err != nil {
		return plugin.ImportDirective{}, err
	}
	defer filePtr.Close()

	if _, err := file.WriteTo(filePtr); err != nil {
		return plugin.ImportDirective{}, err
	}

	return plugin.ImportDirective{
		Resource: tfResourceType,
		Name:     tfResourceName,
		ID:       monitorGUID,
	}, nil
}
