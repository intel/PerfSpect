package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

func (l *StaticLoader) Load(metricDefinitionOverridePath string, eventDefinitionOverridePath string, selectedMetrics []string, metadata Metadata) ([]MetricDefinition, []GroupDefinition, error) {
	loadedMetricDefinitions, err := loadMetricDefinitions(metricDefinitionOverridePath, selectedMetrics, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metric definitions: %w", err)
	}
	loadedEventGroups, uncollectableEvents, err := loadEventGroups(eventDefinitionOverridePath, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load event group definitions: %w", err)
	}
	configuredMetricDefinitions, err := configureMetrics(loadedMetricDefinitions, uncollectableEvents, metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure metrics: %w", err)
	}
	return configuredMetricDefinitions, loadedEventGroups, nil
}

// loadMetricDefinitions reads and parses metric definitions from an architecture-specific metric
// definition file. When the override path argument is empty, the function will load metrics from
// the file associated with the platform's architecture found in the provided metadata. When
// a list of metric names is provided, only those metric definitions will be loaded.
func loadMetricDefinitions(metricDefinitionOverridePath string, selectedMetrics []string, metadata Metadata) (metrics []MetricDefinition, err error) {
	var bytes []byte
	if metricDefinitionOverridePath != "" {
		bytes, err = os.ReadFile(metricDefinitionOverridePath) // #nosec G304
		if err != nil {
			return
		}
	} else {
		uarch := strings.ToLower(strings.Split(metadata.Microarchitecture, "_")[0])
		uarch = strings.Split(uarch, " ")[0]
		// use alternate events/metrics when TMA fixed counters are not supported
		alternate := ""
		if (uarch == "icx" || uarch == "spr" || uarch == "emr" || uarch == "gnr") && !metadata.SupportsFixedTMA {
			alternate = "_nofixedtma"
		}
		metricFileName := fmt.Sprintf("%s%s.json", uarch, alternate)
		if bytes, err = resources.ReadFile(filepath.Join("resources", "metrics", metadata.Architecture, metadata.Vendor, metricFileName)); err != nil {
			return
		}
	}
	var metricsInFile []MetricDefinition
	if err = json.Unmarshal(bytes, &metricsInFile); err != nil {
		return
	}
	// if a list of metric names provided, reduce list to match
	if len(selectedMetrics) > 0 {
		// confirm provided metric names are valid (included in metrics defined in file)
		// and build list of metrics based on provided list of metric names
		metricMap := make(map[string]MetricDefinition)
		for _, metric := range metricsInFile {
			metricMap[metric.Name] = metric
		}
		for _, selectedMetricName := range selectedMetrics {
			if _, ok := metricMap[selectedMetricName]; !ok {
				err = fmt.Errorf("provided metric name not found: %s", selectedMetricName)
				return
			}
			metrics = append(metrics, metricMap[selectedMetricName])
		}
	} else {
		metrics = metricsInFile
	}
	// abbreviate event names in metrics to shorten the eventual perf stat command line
	for i := range metrics {
		metrics[i].Expression = abbreviateEventName(metrics[i].Expression)
	}
	return
}

// loadEventGroups reads the events defined in the architecture specific event definition file, then
// expands them to include the per-device uncore events
func loadEventGroups(eventDefinitionOverridePath string, metadata Metadata) (groups []GroupDefinition, uncollectableEvents []string, err error) {
	var file fs.File
	if eventDefinitionOverridePath != "" {
		file, err = os.Open(eventDefinitionOverridePath) // #nosec G304
		if err != nil {
			return
		}
	} else {
		uarch := strings.ToLower(strings.Split(metadata.Microarchitecture, "_")[0])
		uarch = strings.Split(uarch, " ")[0]
		// use alternate events/metrics when TMA fixed counters are not supported
		alternate := ""
		if (uarch == "icx" || uarch == "spr" || uarch == "emr" || uarch == "gnr") && !metadata.SupportsFixedTMA { // AWS/GCP VM instances
			alternate = "_nofixedtma"
		}
		eventFileName := fmt.Sprintf("%s%s.txt", uarch, alternate)
		if file, err = resources.Open(filepath.Join("resources", "events", metadata.Architecture, metadata.Vendor, eventFileName)); err != nil {
			return
		}
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	uncollectable := mapset.NewSet[string]()
	var group GroupDefinition
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		// strip end of line comment
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}
		// remove trailing spaces
		line = strings.TrimSpace(line)
		var event EventDefinition
		if event, err = parseEventDefinition(line[:len(line)-1]); err != nil {
			return
		}
		// abbreviate the event name to shorten the eventual perf stat command line
		event.Name = abbreviateEventName(event.Name)
		event.Raw = abbreviateEventName(event.Raw)
		if isCollectableEvent(event, metadata) {
			group = append(group, event)
		} else {
			uncollectable.Add(event.Name)
		}
		if line[len(line)-1] == ';' {
			// end of group detected
			if len(group) > 0 {
				groups = append(groups, group)
			} else {
				slog.Debug("No collectable events in group", slog.String("ending", line))
			}
			group = GroupDefinition{} // clear the list
		}
	}
	if err = scanner.Err(); err != nil {
		return
	}
	uncollectableEvents = uncollectable.ToSlice()
	// expand uncore groups for all uncore devices
	groups, err = expandUncoreGroups(groups, metadata)

	if uncollectable.Cardinality() != 0 {
		slog.Debug("Events not collectable on target", slog.String("events", uncollectable.String()))
	}
	return
}

// isCollectableEvent confirms if given event can be collected on the platform
func isCollectableEvent(event EventDefinition, metadata Metadata) bool {
	// fixed-counter TMA
	if !metadata.SupportsFixedTMA && (event.Name == "TOPDOWN.SLOTS" || strings.HasPrefix(event.Name, "PERF_METRICS.")) {
		slog.Debug("Fixed counter TMA not supported on target", slog.String("event", event.Name))
		return false
	}
	// PEBS events (not supported on GCP c4 VMs)
	pebsEventNames := []string{"INT_MISC.UNKNOWN_BRANCH_CYCLES", "UOPS_RETIRED.MS"}
	if !metadata.SupportsPEBS {
		for _, pebsEventName := range pebsEventNames {
			if strings.Contains(event.Name, pebsEventName) {
				slog.Debug("PEBS events not supported on target", slog.String("event", event.Name))
				return false
			}
		}
	}
	// short-circuit for cpu events that aren't off-core response events
	if event.Device == "cpu" && !(strings.HasPrefix(event.Name, "OCR") || strings.HasPrefix(event.Name, "OFFCORE_REQUESTS_OUTSTANDING")) {
		return true
	}
	// off-core response events
	if event.Device == "cpu" && (strings.HasPrefix(event.Name, "OCR") || strings.HasPrefix(event.Name, "OFFCORE_REQUESTS_OUTSTANDING")) {
		if !(metadata.SupportsOCR && metadata.SupportsUncore) {
			slog.Debug("Off-core response events not supported on target", slog.String("event", event.Name))
			return false
		} else if flagScope == scopeProcess || flagScope == scopeCgroup {
			slog.Debug("Off-core response events not supported in process or cgroup scope", slog.String("event", event.Name))
			return false
		}
		return true
	}
	// uncore events
	if !metadata.SupportsUncore && strings.HasPrefix(event.Name, "UNC") {
		slog.Debug("Uncore events not supported on target", slog.String("event", event.Name))
		return false
	}
	// exclude uncore events when
	// - their corresponding device is not found
	// - not in system-wide collection scope
	if event.Device != "cpu" && event.Device != "" {
		if flagScope == scopeProcess || flagScope == scopeCgroup {
			slog.Debug("Uncore events not supported in process or cgroup scope", slog.String("event", event.Name))
			return false
		}
		deviceExists := false
		for uncoreDeviceName := range metadata.UncoreDeviceIDs {
			if event.Device == uncoreDeviceName {
				deviceExists = true
				break
			}
		}
		if !deviceExists {
			slog.Debug("Uncore device not found", slog.String("device", event.Device))
			return false
		} else if !strings.Contains(event.Raw, "umask") && !strings.Contains(event.Raw, "event") {
			slog.Debug("Uncore event missing umask or event", slog.String("event", event.Name))
			return false
		}
		return true
	}
	// if we got this far, event.Device is empty
	// is ref-cycles supported?
	if !metadata.SupportsRefCycles && strings.Contains(event.Name, "ref-cycles") {
		slog.Debug("ref-cycles not supported on target", slog.String("event", event.Name))
		return false
	}
	// no cstate and power events when collecting at process or cgroup scope
	if (flagScope == scopeProcess || flagScope == scopeCgroup) &&
		(strings.Contains(event.Name, "cstate_") || strings.Contains(event.Name, "power/energy")) {
		slog.Debug("Cstate and power events not supported in process or cgroup scope", slog.String("event", event.Name))
		return false
	}
	// finally, if it isn't in the perf list output, it isn't collectable
	name := strings.Split(event.Name, ":")[0]
	if !strings.Contains(metadata.PerfSupportedEvents, name) {
		slog.Debug("Event not supported by perf", slog.String("event", name))
		return false
	}
	return true
}

// parseEventDefinition parses one line from the event definition file into a representative structure
func parseEventDefinition(line string) (eventDef EventDefinition, err error) {
	eventDef.Raw = line
	fields := strings.Split(line, ",")
	if len(fields) == 1 {
		eventDef.Name = fields[0]
	} else if len(fields) > 1 {
		nameField := fields[len(fields)-1]
		if nameField[:5] != "name=" {
			err = fmt.Errorf("unrecognized event format, name field not found: %s", line)
			return
		}
		eventDef.Name = nameField[6 : len(nameField)-2]
		eventDef.Device = strings.Split(fields[0], "/")[0]
	} else {
		err = fmt.Errorf("unrecognized event format: %s", line)
		return
	}
	return
}

// expandUncoreGroup expands a perf event group into a list of groups where each group is
// associated with an uncore device
func expandUncoreGroup(group GroupDefinition, ids []int, re *regexp.Regexp, vendor string) (groups []GroupDefinition, err error) {
	for _, deviceID := range ids {
		var newGroup GroupDefinition
		for _, event := range group {
			match := re.FindStringSubmatch(event.Raw)
			if len(match) == 0 {
				err = fmt.Errorf("unexpected raw event format: %s", event.Raw)
				return
			}
			var newEvent EventDefinition
			if vendor == "AuthenticAMD" {
				newEvent.Name = match[4]
				newEvent.Raw = fmt.Sprintf("amd_%s/event=%s,umask=%s,name='%s'/", match[1], match[2], match[3], newEvent.Name)
			} else {
				newEvent.Name = fmt.Sprintf("%s.%d", match[4], deviceID)
				newEvent.Raw = fmt.Sprintf("uncore_%s_%d/event=%s,umask=%s,name='%s'/", match[1], deviceID, match[2], match[3], newEvent.Name)
			}
			newEvent.Device = event.Device
			newGroup = append(newGroup, newEvent)
		}
		groups = append(groups, newGroup)
	}
	return
}

// expandUncoreGroups expands groups with uncore events to include events for all uncore devices
// assumes that uncore device events are in their own groups, not mixed with other device types
func expandUncoreGroups(groups []GroupDefinition, metadata Metadata) (expandedGroups []GroupDefinition, err error) {
	// example 1: cha/event=0x35,umask=0xc80ffe01,name='UNC_CHA_TOR_INSERTS.IA_MISS_CRD'/,
	// expand to: uncore_cha_0/event=0x35,umask=0xc80ffe01,name='UNC_CHA_TOR_INSERTS.IA_MISS_CRD.0'/,
	// example 2: cha/event=0x36,umask=0x21,config1=0x4043300000000,name='UNC_CHA_TOR_OCCUPANCY.IA_MISS.0x40433'/
	// expand to: uncore_cha_0/event=0x36,umask=0x21,config1=0x4043300000000,name='UNC_CHA_TOR_OCCUPANCY.IA_MISS.0x40433'/
	re := regexp.MustCompile(`(\w+)/event=(0x[0-9,a-f,A-F]+),umask=(0x[0-9,a-f,A-F]+.*),name='(.*)'`)
	var deviceTypes []string
	for deviceType := range metadata.UncoreDeviceIDs {
		deviceTypes = append(deviceTypes, deviceType)
	}
	for _, group := range groups {
		device := group[0].Device
		if slices.Contains(deviceTypes, device) {
			var newGroups []GroupDefinition
			if len(metadata.UncoreDeviceIDs[device]) == 0 {
				slog.Warn("No uncore devices found", slog.String("type", device))
				continue
			}
			if newGroups, err = expandUncoreGroup(group, metadata.UncoreDeviceIDs[device], re, metadata.Vendor); err != nil {
				return
			}
			expandedGroups = append(expandedGroups, newGroups...)
		} else {
			expandedGroups = append(expandedGroups, group)
		}
	}
	return
}

// abbreviateEventName replaces long event names with abbreviations to reduce the length of the perf command.
// focus is on uncore events because they are repeated for each uncore device
func abbreviateEventName(event string) string {
	// Abbreviations must be unique and in order. And, if replacing UNC_*, the abbreviation must begin with "UNC" because this is how we identify uncore events when collapsing them.
	var abbreviations = [][]string{
		{"UNC_CHA_TOR_INSERTS", "UNCCTI"},
		{"UNC_CHA_TOR_OCCUPANCY", "UNCCTO"},
		{"UNC_CHA_CLOCKTICKS", "UNCCCT"},
		{"UNC_M_CAS_COUNT_SCH", "UNCMCC"},
		{"IA_MISS_DRD_REMOTE", "IMDR"},
		{"IA_MISS_DRD_LOCAL", "IMDL"},
		{"IA_MISS_LLCPREFDATA", "IMLP"},
		{"IA_MISS_LLCPREFRFO", "IMLR"},
		{"IA_MISS_DRD_PREF_LOCAL", "IMDPL"},
		{"IA_MISS_DRD_PREF_REMOTE", "IMDRP"},
		{"IA_MISS_CRD_PREF", "IMCP"},
		{"IA_MISS_RFO_PREF", "IMRP"},
		{"IA_MISS_RFO", "IMRF"},
		{"IA_MISS_CRD", "IMC"},
		{"IA_MISS_DRD", "IMD"},
		{"IO_PCIRDCUR", "IPCI"},
		{"IO_ITOMCACHENEAR", "IITN"},
		{"IO_ITOM", "IITO"},
		{"IMD_OPT", "IMDO"},
	}
	// if an abbreviation key is found in the event, replace the matching portion of the event with the abbreviation
	for _, abbr := range abbreviations {
		event = strings.Replace(event, abbr[0], abbr[1], -1)
	}
	return event
}
