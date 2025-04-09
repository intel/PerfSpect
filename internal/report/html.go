package report

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bytes"
	"fmt"
	"html"
	htmltemplate "html/template"
	"log/slog"
	"math"
	"math/rand/v2" // nosemgrep
	"slices"
	"sort"
	"strconv"
	"strings"
	texttemplate "text/template" // nosemgrep
)

func getHtmlReportBegin() string {
	var sb strings.Builder
	sb.WriteString(`<!--
 * Copyright (C) 2024 Intel Corporation
 * SPDX-License-Identifier: MIT
-->
`)
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
`)
	sb.WriteString("<head>\n")
	sb.WriteString(`    <meta charset="UTF-8">
    <title>Intel&reg; PerfSpect</title>
    <link rel="icon" type="image/x-icon" href="https://www.intel.com/favicon.ico">
    <meta name="viewport" content="width=device-width, initial-scale=1">
`)
	// link the style sheets and javascript
	sb.WriteString(`
	<link rel="stylesheet" href="https://unpkg.com/normalize.css@8.0.1/normalize.css" integrity="sha384-M86HUGbBFILBBZ9ykMAbT3nVb0+2C7yZlF8X2CiKNpDOQjKroMJqIeGZ/Le8N2Qp" crossorigin="anonymous" referrerpolicy="no-referrer" />
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/purecss@3.0.0/build/pure-min.css" integrity="sha384-X38yfunGUhNzHpBaEBsWLO+A0HDYOQi8ufWDkZ0k9e0eXz/tH3II7uKZ9msv++Ls" crossorigin="anonymous" referrerpolicy="no-referrer" />
    <script src="https://unpkg.com/chart.js@3.7.1/dist/chart.min.js" integrity="sha384-7NrRHqlWUj2hJl3a/dZj/a1GxuQc56mJ3aYsEnydBYrY1jR+RSt6SBvK3sHfj+mJ" crossorigin="anonymous"  referrerpolicy="no-referrer"></script>
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css">
	<link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/d3-flame-graph@4.1.3/dist/d3-flamegraph.css">
    <script type="text/javascript" src="https://d3js.org/d3.v7.js"></script>
    <script type="text/javascript" src="https://cdn.jsdelivr.net/npm/d3-flame-graph@4.1.3/dist/d3-flamegraph.min.js"></script>
	`)
	// add content class style
	sb.WriteString(`
	<style>
        .content {
            padding: 0 2em;
            line-height: 1.6em;
        }
        .content h2 {
            font-weight: 300;
            color: #888;
        }
        .content h2:before {
            content: '';
            display: block;
            position: relative;
            width: 0;
            height: 5em;
            margin-top: -5em
        }
	</style>
`)
	// add sidebar class styles
	sb.WriteString(`
	<style>
		.sidebar {
            height: 100%;
            width: 0;
            position: fixed;
            z-index: 1;
            top: 0;
            left: 0;
            background-color: #111;
            overflow-x: hidden;
            transition: 0.5s;
            padding-top: 30px;
            padding-left: 0px;
        }
        .sidebar h1 {
            position: absolute;
            top: 0;
            padding: 0px 8px 8px 35px;
            text-decoration: none;
            color: #fff;
            background-color: #1f8dd6;
            display: block;
            transition: 0.3s;
        }
		.sidebar h2 {
            padding: 8px 4px 2px 35px;
            text-decoration: none;
            color: #818181;
            display: block;
		}
        .sidebar a {
            padding: 0px 4px 2px 35px;
            text-decoration: none;
            color: #818181;
            display: block;
            transition: 0.3s;
        }
        .sidebar a:hover {
            color: #f1f1f1;
        }
        .sidebar .togglebtn {
            position: absolute;
            top: 0;
            right: 0px;
            font-size: 25px;
            padding-left: 5px;
            color: #ccc;
            background-color: #1f8dd6;
        }
        .sidebar .togglebtn:hover {
            color: #666;
        }
	</style>
	`)
	sb.WriteString("</head>\n")

	return sb.String()
}

func getHtmlReportMenu(allTableValues []TableValues) string {
	var sb strings.Builder
	// if none of the tables have menu labels, don't add the sidebar
	hasMenuLabels := false
	for _, tableValues := range allTableValues {
		if tableValues.MenuLabel != "" {
			hasMenuLabels = true
			break
		}
	}
	if hasMenuLabels {
		sb.WriteString("<div id=\"mySidebar\" class=\"sidebar\">\n")
		sb.WriteString("<a href=\"#\" style=\"position: absolute;top: 0; padding-left: 7px; padding-right: 117px; color: #fff; background-color: #1f8dd6\">CONTENTS</a>\n")
		sb.WriteString("<a href=\"javascript:void(0)\" class=\"togglebtn\" onclick=\"toggleNav()\">&lt;</a>\n")
		// insert menu items into sidebar
		for _, tableValues := range allTableValues {
			if tableValues.MenuLabel != "" {
				sb.WriteString(fmt.Sprintf("<a href=\"#%s\">%s</a>\n", html.EscapeString(tableValues.Name), html.EscapeString(tableValues.MenuLabel)))
			}
		}
		sb.WriteString("</div>\n") // end of sidebar
	}
	return sb.String()
}

func getHtmlReportSidebarJavascript() string {
	return `
	<script>
		const widthOpen="225px"
		const widthClosed="30px"
		function openNav() {
			document.getElementById("mySidebar").style.width = widthOpen;
			document.getElementById("myTables").style.marginLeft = widthOpen;
			document.querySelector(".togglebtn").innerHTML="<"
		}
		function closeNav() {
			document.getElementById("mySidebar").style.width = widthClosed;
			document.getElementById("myTables").style.marginLeft= widthClosed;
			document.querySelector(".togglebtn").innerHTML=">"
		}
		function toggleNav() {
			if (document.getElementById("mySidebar").style.width !== widthOpen) {
				openNav()
			} else {
				closeNav()
			}
		}
		// open on startup
		openNav()
	</script>
	`
}

func createHtmlReport(allTableValues []TableValues, targetName string) (out []byte, err error) {
	var sb strings.Builder
	sb.WriteString(getHtmlReportBegin())

	// body starts here
	sb.WriteString("<body>\n")
	sb.WriteString("<main class=\"content\">\n")
	// add the sidebar/menu
	sb.WriteString(getHtmlReportMenu(allTableValues))
	// add the tables
	sb.WriteString("<div id=\"myTables\">\n")
	sb.WriteString("<h1>Intel&reg; PerfSpect</h1>\n")
	sb.WriteString(`                    
<noscript>
	<h3>JavaScript is disabled. Functionality is limited.</h3>
</noscript>
`)
	for _, tableValues := range allTableValues {
		// print the table name
		sb.WriteString(fmt.Sprintf("<h2 id=\"%[1]s\">%[1]s</h2>\n", html.EscapeString(tableValues.Name)))
		// if there's no data in the table, print a message and continue
		if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
			msg := noDataFound
			if tableValues.NoDataFound != "" {
				msg = tableValues.NoDataFound
			}
			sb.WriteString("<p>" + msg + "</p>\n")
			continue
		}
		// render the tables
		if tableValues.HTMLTableRendererFunc != nil { // custom table renderer
			sb.WriteString(tableValues.HTMLTableRendererFunc(tableValues, targetName))
		} else {
			sb.WriteString(DefaultHTMLTableRendererFunc(tableValues))
		}
	}
	sb.WriteString("</div>\n") // end of myTables
	sb.WriteString("</main>\n")

	// add the sidebar toggle function
	sb.WriteString(getHtmlReportSidebarJavascript())

	sb.WriteString("</body>\n")
	sb.WriteString("</html>\n")
	out = []byte(sb.String())
	return
}

func createHtmlReportMultiTarget(allTargetsTableValues [][]TableValues, targetNames []string, allTableNames []string) (out []byte, err error) {
	var sb strings.Builder
	sb.WriteString(getHtmlReportBegin())

	// body starts here
	sb.WriteString("<body>\n")
	sb.WriteString("<main class=\"content\">\n")
	// add the sidebar/menu
	sb.WriteString(getHtmlReportMenu(allTargetsTableValues[0]))
	// add the tables
	sb.WriteString("<div id=\"myTables\">\n")
	sb.WriteString("<h1>Intel&reg; PerfSpect</h1>\n")
	sb.WriteString(`                    
<noscript>
	<h3>JavaScript is disabled. Functionality is limited.</h3>
</noscript>
`)
	// print the tables in the order they were passed in
	for _, tableName := range allTableNames {
		oneTableValuesForAllTargets := []TableValues{}
		// build list of target names and TableValues for targets that have values for this table
		tableTargets := []string{}
		tableValues := []TableValues{}
		for targetIndex, targetTableValues := range allTargetsTableValues {
			tableIndex := findTableIndex(targetTableValues, tableName)
			if tableIndex == -1 {
				continue
			}
			tableTargets = append(tableTargets, targetNames[targetIndex])
			tableValues = append(tableValues, targetTableValues[tableIndex])
		}
		// loop through the targets that have values for this table
		for targetIndex, targetTableValues := range tableValues {
			targetName := tableTargets[targetIndex]
			// if the table has rows and no custom renderer, print the table for the target normally
			if targetTableValues.HasRows && targetTableValues.HTMLMultiTargetTableRendererFunc == nil {
				// print the table name only one time per table
				if targetIndex == 0 {
					sb.WriteString(fmt.Sprintf("<h2 id=\"%[1]s\">%[1]s</h2>\n", html.EscapeString(targetTableValues.Name)))
				}
				// print the target name
				sb.WriteString(fmt.Sprintf("<h3>%s</h3>\n", targetName))
				// if there's no data in the table, print a message and continue
				if len(targetTableValues.Fields) == 0 || len(targetTableValues.Fields[0].Values) == 0 {
					sb.WriteString("<p>" + noDataFound + "</p>\n")
					continue
				}
				if targetTableValues.HTMLTableRendererFunc != nil { // custom table renderer
					sb.WriteString(targetTableValues.HTMLTableRendererFunc(targetTableValues, targetNames[targetIndex]))
				} else {
					sb.WriteString(DefaultHTMLTableRendererFunc(targetTableValues))
				}
			} else { // if the table has no rows or a custom renderer, add the table to the list to render as a multi-target table
				oneTableValuesForAllTargets = append(oneTableValuesForAllTargets, targetTableValues)
			}
		}
		// print the multi-target table, if any
		if len(oneTableValuesForAllTargets) > 0 {
			sb.WriteString(fmt.Sprintf("<h2 id=\"%[1]s\">%[1]s</h2>\n", html.EscapeString(oneTableValuesForAllTargets[0].Name)))
			if oneTableValuesForAllTargets[0].HTMLMultiTargetTableRendererFunc != nil {
				sb.WriteString(oneTableValuesForAllTargets[0].HTMLMultiTargetTableRendererFunc(oneTableValuesForAllTargets, targetNames))
			} else {
				// render the multi-target table
				sb.WriteString(RenderMultiTargetTableValuesAsHTML(oneTableValuesForAllTargets, targetNames))
			}
		}
	}
	sb.WriteString("</div>\n") // end of myTables
	sb.WriteString("</main>\n")

	// add the sidebar toggle function
	sb.WriteString(getHtmlReportSidebarJavascript())

	sb.WriteString("</body>\n")
	sb.WriteString("</html>\n")
	out = []byte(sb.String())
	return
}

// findTableIndex
func findTableIndex(tableValues []TableValues, tableName string) int {
	for i, tableValue := range tableValues {
		if tableValue.Name == tableName {
			return i
		}
	}
	return -1
}

const datasetTemplate = `
{
	label: '{{.Label}}',
	data: [{{.Data}}],
	backgroundColor: '{{.Color}}',
	borderColor: '{{.Color}}',
	borderWidth: 1,
	showLine: true
}
`
const lineChartTemplate = `<div class="chart-container" style="max-width: 900px">
<canvas id="{{.ID}}"></canvas>
</div>
<script>
new Chart(document.getElementById('{{.ID}}'), {
    type: 'line',
    data: {
		labels: [{{.Labels}}],
        datasets: [{{.Datasets}}]
    },
    options: {
        aspectRatio: {{.AspectRatio}},
        scales: {
            x: {
                beginAtZero: false,
                title: {
                    text: "{{.XaxisText}}",
                    display: true
                },
				ticks: {
					maxRotation: 90,
					minRotation: 45
                }
            },
            y: {
                title: {
                    text: "{{.YaxisText}}",
                    display: true
                },
				suggestedMin: {{.SuggestedMin}},
				suggestedMax: {{.SuggestedMax}},
            }
        },
        plugins: {
            title: {
                text: "{{.TitleText}}",
                display: {{.DisplayTitle}},
                font: {
                    size: 18
                }
            },
            tooltip: {
                callbacks: {
                    label: function(ctx) {
                        return ctx.dataset.label + " (" + ctx.parsed.x + ", " + ctx.parsed.y + ")";
                    }
                }
            },
            legend: {
                display: {{.DisplayLegend}}
            }
        }
    }
});
</script>
`
const scatterChartTemplate = `<div class="chart-container" style="max-width: 900px">
<canvas id="{{.ID}}"></canvas>
</div>
<script>
new Chart(document.getElementById('{{.ID}}'), {
    type: 'scatter',
    data: {
        datasets: [{{.Datasets}}]
    },
    options: {
        aspectRatio: {{.AspectRatio}},
        scales: {
            x: {
                beginAtZero: false,
                title: {
                    text: "{{.XaxisText}}",
                    display: true
                }
            },
            y: {
                title: {
                    text: "{{.YaxisText}}",
                    display: true
                },
				suggestedMin: {{.SuggestedMin}},
				suggestedMax: {{.SuggestedMax}},
            }
        },
        plugins: {
            title: {
                text: "{{.TitleText}}",
                display: {{.DisplayTitle}},
                font: {
                    size: 18
                }
            },
            tooltip: {
                callbacks: {
                    label: function(ctx) {
                        return ctx.dataset.label + " (" + ctx.parsed.x + ", " + ctx.parsed.y + ")";
                    }
                }
            },
            legend: {
                display: {{.DisplayLegend}}
            }
        }
    }
});
</script>
`

type chartTemplateStruct struct {
	ID            string
	Labels        string // only for line charts
	Datasets      string
	XaxisText     string
	YaxisText     string
	TitleText     string
	DisplayTitle  string
	DisplayLegend string
	AspectRatio   string
	SuggestedMin  string
	SuggestedMax  string
}

func renderHTMLTable(tableHeaders []string, tableValues [][]string, class string, valuesStyle [][]string) string {
	var sb strings.Builder
	sb.WriteString(`<table class="` + class + `">`)
	if len(tableHeaders) > 0 {
		sb.WriteString(`<thead>`)
		sb.WriteString(`<tr>`)
		for _, label := range tableHeaders {
			sb.WriteString(`<th>` + label + `</th>`)
		}
		sb.WriteString(`</tr>`)
		sb.WriteString(`</thead>`)
	}
	sb.WriteString(`<tbody>`)
	for rowIdx, rowValues := range tableValues {
		sb.WriteString(`<tr>`)
		for colIdx, value := range rowValues {
			var style string
			if len(valuesStyle) > rowIdx && len(valuesStyle[rowIdx]) > colIdx {
				style = ` style="` + valuesStyle[rowIdx][colIdx] + `"`
			}
			sb.WriteString(`<td` + style + `>` + value + `</td>`)
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody>`)
	sb.WriteString(`</table>`)
	return sb.String()
}

func DefaultHTMLTableRendererFunc(tableValues TableValues) string {
	if tableValues.HasRows { // print the field names as column headings across the top of the table
		headers := []string{}
		for _, field := range tableValues.Fields {
			headers = append(headers, field.Name)
		}
		values := [][]string{}
		for row := range tableValues.Fields[0].Values {
			rowValues := []string{}
			for _, field := range tableValues.Fields {
				rowValues = append(rowValues, htmltemplate.HTMLEscapeString(field.Values[row]))
			}
			values = append(values, rowValues)
		}
		return renderHTMLTable(headers, values, "pure-table pure-table-striped", [][]string{})
	} else { // print the field name followed by its value
		values := [][]string{}
		var tableValueStyles [][]string
		for _, field := range tableValues.Fields {
			rowValues := []string{}
			rowValues = append(rowValues, field.Name)
			rowValues = append(rowValues, htmltemplate.HTMLEscapeString(field.Values[0]))
			values = append(values, rowValues)
			tableValueStyles = append(tableValueStyles, []string{"font-weight:bold"})
		}
		return renderHTMLTable([]string{}, values, "pure-table pure-table-striped", tableValueStyles)
	}
}

// RenderMultiTargetTableValuesAsHTML renders a table for multiple targets
// tableValues is a slice of TableValues, each of which represents the same table from a single target
func RenderMultiTargetTableValuesAsHTML(tableValues []TableValues, targetNames []string) string {
	values := [][]string{}
	var tableValueStyles [][]string
	for fieldIndex, field := range tableValues[0].Fields {
		rowValues := []string{}
		rowValues = append(rowValues, field.Name)
		for _, targetTableValues := range tableValues {
			if len(targetTableValues.Fields) > fieldIndex && len(targetTableValues.Fields[fieldIndex].Values) > 0 {
				rowValues = append(rowValues, htmltemplate.HTMLEscapeString(targetTableValues.Fields[fieldIndex].Values[0]))
			} else {
				rowValues = append(rowValues, "")
			}
		}
		values = append(values, rowValues)
		tableValueStyles = append(tableValueStyles, []string{"font-weight:bold"})
	}
	headers := []string{""}
	headers = append(headers, targetNames...)
	return renderHTMLTable(headers, values, "pure-table pure-table-striped", tableValueStyles)
}

func dimmDetails(dimm []string) (details string) {
	if strings.Contains(dimm[SizeIdx], "No") {
		details = "No Module Installed"
	} else {
		// Intel PMEM modules may have serial number appended to end of part number...
		// strip that off so it doesn't mess with color selection later
		partNumber := dimm[PartIdx]
		if strings.Contains(dimm[DetailIdx], "Synchronous Non-Volatile") &&
			dimm[ManufacturerIdx] == "Intel" &&
			strings.HasSuffix(dimm[PartIdx], dimm[SerialIdx]) {
			partNumber = dimm[PartIdx][:len(dimm[PartIdx])-len(dimm[SerialIdx])]
		}
		// example: "64GB DDR5 R2 Synchronous Registered (Buffered) Micron Technology MTC78ASF4G72PZ-2G6E1 6400 MT/s [6000 MT/s]"
		details = fmt.Sprintf("%s %s %s R%s %s %s %s [%s]",
			strings.ReplaceAll(dimm[SizeIdx], " ", ""),
			dimm[TypeIdx],
			dimm[DetailIdx],
			dimm[RankIdx],
			dimm[ManufacturerIdx],
			partNumber,
			strings.ReplaceAll(dimm[SpeedIdx], " ", ""),
			strings.ReplaceAll(dimm[ConfiguredSpeedIdx], " ", ""))
	}
	return
}

func dimmTableHTMLRenderer(tableValues TableValues, targetName string) string {
	if tableValues.Fields[DerivedSocketIdx].Values[0] == "" || tableValues.Fields[DerivedChannelIdx].Values[0] == "" || tableValues.Fields[DerivedSlotIdx].Values[0] == "" {
		return DefaultHTMLTableRendererFunc(tableValues)
	}
	htmlColors := []string{"lightgreen", "orange", "aqua", "lime", "yellow", "beige", "magenta", "violet", "salmon", "pink"}
	var slotColorIndices = make(map[string]int)
	// socket -> channel -> slot -> dimm details
	var dimms = map[string]map[string]map[string]string{}
	for dimmIdx := range tableValues.Fields[DerivedSocketIdx].Values {
		if _, ok := dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]]; !ok {
			dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]] = make(map[string]map[string]string)
		}
		if _, ok := dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]][tableValues.Fields[DerivedChannelIdx].Values[dimmIdx]]; !ok {
			dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]][tableValues.Fields[DerivedChannelIdx].Values[dimmIdx]] = make(map[string]string)
		}
		dimmValues := []string{}
		for _, field := range tableValues.Fields {
			dimmValues = append(dimmValues, field.Values[dimmIdx])
		}
		dimms[tableValues.Fields[DerivedSocketIdx].Values[dimmIdx]][tableValues.Fields[DerivedChannelIdx].Values[dimmIdx]][tableValues.Fields[DerivedSlotIdx].Values[dimmIdx]] = dimmDetails(dimmValues)
	}

	var socketTableHeaders = []string{"Socket", ""}
	var socketTableValues [][]string
	var socketKeys []string
	for k := range dimms {
		socketKeys = append(socketKeys, k)
	}
	sort.Strings(socketKeys)
	for _, socket := range socketKeys {
		socketMap := dimms[socket]
		socketTableValues = append(socketTableValues, []string{})
		var channelTableHeaders = []string{"Channel", "Slots"}
		var channelTableValues [][]string
		var channelKeys []int
		for k := range socketMap {
			channel, err := strconv.Atoi(k)
			if err != nil {
				slog.Error("failed to convert channel to int", slog.String("error", err.Error()))
				return ""
			}
			channelKeys = append(channelKeys, channel)
		}
		sort.Ints(channelKeys)
		for _, channel := range channelKeys {
			channelMap := socketMap[strconv.Itoa(channel)]
			channelTableValues = append(channelTableValues, []string{})
			var slotTableHeaders []string
			var slotTableValues [][]string
			var slotTableValuesStyles [][]string
			var slotKeys []string
			for k := range channelMap {
				slotKeys = append(slotKeys, k)
			}
			sort.Strings(slotKeys)
			slotTableValues = append(slotTableValues, []string{})
			slotTableValuesStyles = append(slotTableValuesStyles, []string{})
			for _, slot := range slotKeys {
				dimmDetails := channelMap[slot]
				slotTableValues[0] = append(slotTableValues[0], htmltemplate.HTMLEscapeString(dimmDetails))
				var slotColor string
				if dimmDetails == "No Module Installed" {
					slotColor = "background-color:silver"
				} else {
					if _, ok := slotColorIndices[dimmDetails]; !ok {
						slotColorIndices[dimmDetails] = int(math.Min(float64(len(slotColorIndices)), float64(len(htmlColors)-1)))
					}
					slotColor = "background-color:" + htmlColors[slotColorIndices[dimmDetails]]
				}
				slotTableValuesStyles[0] = append(slotTableValuesStyles[0], slotColor)
			}
			slotTable := renderHTMLTable(slotTableHeaders, slotTableValues, "pure-table pure-table-bordered", slotTableValuesStyles)
			// channel number
			channelTableValues[len(channelTableValues)-1] = append(channelTableValues[len(channelTableValues)-1], strconv.Itoa(channel))
			// slot table
			channelTableValues[len(channelTableValues)-1] = append(channelTableValues[len(channelTableValues)-1], slotTable)
			// style
		}
		channelTable := renderHTMLTable(channelTableHeaders, channelTableValues, "pure-table pure-table-bordered", [][]string{})
		// socket number
		socketTableValues[len(socketTableValues)-1] = append(socketTableValues[len(socketTableValues)-1], socket)
		// channel table
		socketTableValues[len(socketTableValues)-1] = append(socketTableValues[len(socketTableValues)-1], channelTable)
	}
	return renderHTMLTable(socketTableHeaders, socketTableValues, "pure-table pure-table-bordered", [][]string{})
}

func renderChart(chartType string, allFormattedPoints []string, datasetNames []string, xAxisLabels []string, config chartTemplateStruct) string {
	datasets := []string{}
	for dataIdx, formattedPoints := range allFormattedPoints {
		specValues := formattedPoints
		dst := texttemplate.Must(texttemplate.New("datasetTemplate").Parse(datasetTemplate))
		buf := new(bytes.Buffer)
		err := dst.Execute(buf, struct {
			Label string
			Data  string
			Color string
		}{
			Label: datasetNames[dataIdx],
			Data:  specValues,
			Color: getColor(dataIdx),
		})
		if err != nil {
			slog.Error("error executing template", slog.String("error", err.Error()))
			return "Error rendering chart."
		}
		datasets = append(datasets, buf.String())
	}
	var chartTemplate string
	if chartType == "line" {
		chartTemplate = lineChartTemplate
	} else if chartType == "scatter" {
		chartTemplate = scatterChartTemplate
	} else {
		panic("unknown chart type")
	}
	sct := texttemplate.Must(texttemplate.New("chartTemplate").Parse(chartTemplate))
	buf := new(bytes.Buffer)
	config.Datasets = strings.Join(datasets, ",")
	if chartType == "line" {
		config.Labels = func() string {
			var labels []string
			for _, label := range xAxisLabels {
				labels = append(labels, fmt.Sprintf("'%s'", label))
			}
			return strings.Join(labels, ",")
		}()
	}
	err := sct.Execute(buf, config)
	if err != nil {
		slog.Error("error executing template", slog.String("error", err.Error()))
		return "Error rendering chart."
	}
	out := buf.String()
	out += "\n"
	return out
}

type scatterPoint struct {
	x float64
	y float64
}

func renderScatterChart(data [][]scatterPoint, datasetNames []string, config chartTemplateStruct) string {
	allFormattedPoints := []string{}
	for dataIdx := range data {
		formattedPoints := []string{}
		for _, point := range data[dataIdx] {
			formattedPoints = append(formattedPoints, fmt.Sprintf("{x: %f, y: %f}", point.x, point.y))
		}
		allFormattedPoints = append(allFormattedPoints, strings.Join(formattedPoints, ","))
	}
	return renderChart("scatter", allFormattedPoints, datasetNames, nil, config)
}

func renderLineChart(xAxisLabels []string, data [][]float64, datasetNames []string, config chartTemplateStruct) string {
	allFormattedPoints := []string{}
	for dataIdx := range data {
		formattedPoints := []string{}
		for _, point := range data[dataIdx] {
			formattedPoints = append(formattedPoints, fmt.Sprintf("%f", point))
		}
		allFormattedPoints = append(allFormattedPoints, strings.Join(formattedPoints, ","))
	}
	return renderChart("line", allFormattedPoints, datasetNames, xAxisLabels, config)
}

func renderFrequencyTable(tableValues TableValues) (out string) {
	var rows [][]string
	headers := []string{""}
	valuesStyles := [][]string{}
	for i := range tableValues.Fields[0].Values {
		headers = append(headers, fmt.Sprintf("%d", i+1))
	}
	for _, field := range tableValues.Fields[1:] {
		row := append([]string{field.Name}, field.Values...)
		rows = append(rows, row)
		valuesStyles = append(valuesStyles, []string{"font-weight:bold"})
	}
	out = renderHTMLTable(headers, rows, "pure-table pure-table-striped", valuesStyles)
	return
}

func coreTurboFrequencyTableHTMLRenderer(tableValues TableValues) string {
	data := [][]scatterPoint{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []scatterPoint{}
		for i, val := range field.Values {
			if val == "" {
				break
			}
			freq, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing frequency", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, scatterPoint{float64(i + 1), freq})
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("turboFrequency%d", rand.IntN(10000)),
		XaxisText:     "Core Count",
		YaxisText:     "Frequency (GHz)",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "4",
		SuggestedMin:  "2",
		SuggestedMax:  "4",
	}
	out := renderScatterChart(data, datasetNames, chartConfig)
	out += "\n"
	out += renderFrequencyTable(tableValues)
	return out
}

func cpuFrequencyTableHtmlRenderer(tableValues TableValues, targetName string) string {
	return coreTurboFrequencyTableHTMLRenderer(tableValues)
}

func memoryLatencyTableHtmlRenderer(tableValues TableValues, targetName string) string {
	return memoryLatencyTableMultiTargetHtmlRenderer([]TableValues{tableValues}, []string{targetName})
}

func memoryLatencyTableMultiTargetHtmlRenderer(allTableValues []TableValues, targetNames []string) string {
	data := [][]scatterPoint{}
	datasetNames := []string{}
	for targetIdx, tableValues := range allTableValues {
		points := []scatterPoint{}
		for valIdx := range tableValues.Fields[0].Values {
			latency, err := strconv.ParseFloat(tableValues.Fields[0].Values[valIdx], 64)
			if err != nil {
				slog.Error("error parsing latency", slog.String("error", err.Error()))
				return ""
			}
			bandwidth, err := strconv.ParseFloat(tableValues.Fields[1].Values[valIdx], 64)
			if err != nil {
				slog.Error("error parsing bandwidth", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, scatterPoint{bandwidth, latency})
		}
		data = append(data, points)
		datasetNames = append(datasetNames, targetNames[targetIdx])
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("latencyBandwidth%d", rand.IntN(10000)),
		XaxisText:     "Bandwidth (MB/s)",
		YaxisText:     "Latency (ns)",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "4",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return renderScatterChart(data, datasetNames, chartConfig)
}

func getColor(idx int) string {
	// color-blind safe palette from here: http://mkweb.bcgsc.ca/colorblind/palettes.mhtml#page-container
	colors := []string{"#9F0162", "#009F81", "#FF5AAF", "#00FCCF", "#8400CD", "#008DF9", "#00C2F9", "#FFB2FD", "#A40122", "#E20134", "#FF6E3A", "#FFC33B"}
	return colors[idx%len(colors)]
}

func telemetryTableHTMLRenderer(tableValues TableValues, data [][]float64, datasetNames []string, chartConfig chartTemplateStruct) string {
	tsFieldIdx := 0
	var timestamps []string
	for i := range tableValues.Fields[0].Values {
		timestamp := tableValues.Fields[tsFieldIdx].Values[i]
		if !slices.Contains(timestamps, timestamp) { // could be slow if list is long
			timestamps = append(timestamps, timestamp)
		}
	}
	return renderLineChart(timestamps, data, datasetNames, chartConfig)
}

func cpuUtilizationTableHTMLRenderer(tableValues TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	// collect the busy (100 - idle) values for each CPU
	cpuBusyStats := make(map[int][]float64)
	idleFieldIdx := len(tableValues.Fields) - 1
	cpuFieldIdx := 1
	for i := range tableValues.Fields[0].Values {
		idle, err := strconv.ParseFloat(tableValues.Fields[idleFieldIdx].Values[i], 64)
		if err != nil {
			continue
		}
		busy := 100 - idle
		cpu, err := strconv.Atoi(tableValues.Fields[cpuFieldIdx].Values[i])
		if err != nil {
			continue
		}
		if _, ok := cpuBusyStats[cpu]; !ok {
			cpuBusyStats[cpu] = []float64{}
		}
		cpuBusyStats[cpu] = append(cpuBusyStats[cpu], busy)
	}
	// sort map keys by cpu number
	var keys []int
	for cpu := range cpuBusyStats {
		keys = append(keys, cpu)
	}
	sort.Ints(keys)
	// build the data
	for _, cpu := range keys {
		if len(cpuBusyStats[cpu]) > 0 {
			data = append(data, cpuBusyStats[cpu])
			datasetNames = append(datasetNames, fmt.Sprintf("CPU %d", cpu))
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     "% Utilization",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "false",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "100",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

func summaryCPUUtilizationTableHTMLRenderer(tableValues TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			util, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing percentage", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, util)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     "% Utilization",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "100",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

func irqRateTableHTMLRenderer(tableValues TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[2:] { // 1 data set per field, e.g., %usr, %nice, etc., skip Time and CPU fields
		datasetNames = append(datasetNames, field.Name)
		// sum the values in the field per timestamp, store the sum as a point
		timeStamp := tableValues.Fields[0].Values[0]
		points := []float64{}
		total := 0.0
		for i := range field.Values {
			if tableValues.Fields[0].Values[i] != timeStamp { // new timestamp?
				points = append(points, total)
				total = 0.0
				timeStamp = tableValues.Fields[0].Values[i]
			}
			val, err := strconv.ParseFloat(field.Values[i], 64)
			if err != nil {
				slog.Error("error parsing value", slog.String("error", err.Error()))
				return ""
			}
			total += val
		}
		points = append(points, total) // add the point for the last timestamp
		// save the points in the data slice
		data = append(data, points)
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     "IRQ/s",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

// driveStatsTableHTMLRenderer renders charts of drive statistics
// - one scatter chart per drive, showing the drive's utilization over time
// - each drive stat is a separate dataset within the chart
func driveStatsTableHTMLRenderer(tableValues TableValues, targetName string) string {
	var out string
	driveStats := make(map[string][][]string)
	for i := range tableValues.Fields[0].Values {
		drive := tableValues.Fields[1].Values[i]
		if _, ok := driveStats[drive]; !ok {
			driveStats[drive] = make([][]string, len(tableValues.Fields)-2)
		}
		for j := range len(tableValues.Fields) - 2 {
			driveStats[drive][j] = append(driveStats[drive][j], tableValues.Fields[j+2].Values[i])
		}
	}
	var keys []string
	for drive := range driveStats {
		keys = append(keys, drive)
	}
	sort.Strings(keys)
	for _, drive := range keys {
		data := [][]float64{}
		datasetNames := []string{}
		for i, statVals := range driveStats[drive] {
			points := []float64{}
			for i, val := range statVals {
				if val == "" {
					slog.Error("empty stat value", slog.String("drive", drive), slog.Int("index", i))
					return ""
				}
				util, err := strconv.ParseFloat(val, 64)
				if err != nil {
					slog.Error("error parsing stat", slog.String("error", err.Error()))
					return ""
				}
				points = append(points, util)
			}
			if len(points) > 0 {
				data = append(data, points)
				datasetNames = append(datasetNames, tableValues.Fields[i+2].Name)
			}
		}
		chartConfig := chartTemplateStruct{
			ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
			XaxisText:     "Time",
			YaxisText:     "",
			TitleText:     drive,
			DisplayTitle:  "true",
			DisplayLegend: "true",
			AspectRatio:   "2",
			SuggestedMin:  "0",
			SuggestedMax:  "0",
		}
		out += telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
	}
	return out
}

// networkStatsTableHTMLRenderer renders charts of network device statistics
// - one scatter chart per network device, showing the device's utilization over time
// - each network stat is a separate dataset within the chart
func networkStatsTableHTMLRenderer(tableValues TableValues, targetName string) string {
	var out string
	nicStats := make(map[string][][]string)
	for i := range tableValues.Fields[0].Values {
		drive := tableValues.Fields[1].Values[i]
		if _, ok := nicStats[drive]; !ok {
			nicStats[drive] = make([][]string, len(tableValues.Fields)-2)
		}
		for j := range len(tableValues.Fields) - 2 {
			nicStats[drive][j] = append(nicStats[drive][j], tableValues.Fields[j+2].Values[i])
		}
	}
	var keys []string
	for drive := range nicStats {
		keys = append(keys, drive)
	}
	sort.Strings(keys)
	for _, nic := range keys {
		data := [][]float64{}
		datasetNames := []string{}
		for i, statVals := range nicStats[nic] {
			points := []float64{}
			for i, val := range statVals {
				if val == "" {
					slog.Error("empty stat value", slog.String("nic", nic), slog.Int("index", i))
					return ""
				}
				util, err := strconv.ParseFloat(val, 64)
				if err != nil {
					slog.Error("error parsing stat", slog.String("error", err.Error()))
					return ""
				}
				points = append(points, util)
			}
			if len(points) > 0 {
				data = append(data, points)
				datasetNames = append(datasetNames, tableValues.Fields[i+2].Name)
			}
		}
		chartConfig := chartTemplateStruct{
			ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
			XaxisText:     "Time",
			YaxisText:     "",
			TitleText:     nic,
			DisplayTitle:  "true",
			DisplayLegend: "true",
			AspectRatio:   "2",
			SuggestedMin:  "0",
			SuggestedMax:  "0",
		}
		out += telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
	}
	return out
}

func memoryStatsTableHTMLRenderer(tableValues TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     "kilobytes",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

func summaryCpuFreqTelemetryTableHTMLRenderer(tableValues TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     "MHz",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

func powerStatsTableHTMLRenderer(tableValues TableValues, targetName string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     "Watts",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

func instructionMixTableHTMLRenderer(tableValues TableValues, targetname string) string {
	data := [][]float64{}
	datasetNames := []string{}
	for _, field := range tableValues.Fields[1:] {
		points := []float64{}
		for _, val := range field.Values {
			if val == "" {
				break
			}
			stat, err := strconv.ParseFloat(val, 64)
			if err != nil {
				slog.Error("error parsing stat", slog.String("error", err.Error()))
				return ""
			}
			points = append(points, stat)
		}
		if len(points) > 0 {
			data = append(data, points)
			datasetNames = append(datasetNames, field.Name)
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     "% Samples",
		TitleText:     "",
		DisplayTitle:  "false",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  "0",
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

func renderGaudiStatsChart(tableValues TableValues, chartStatFieldName string, titleText string, yAxisText string, suggestedMax string) string {
	data := [][]float64{}
	datasetNames := []string{}
	// timestamp is in the first field
	// find the module_id field index
	moduleIdFieldIdx, err := getFieldIndex("module_id", tableValues)
	if err != nil {
		slog.Error("no gaudi module_id field found")
		return ""
	}
	// find the chartStatFieldName field index
	chartStatFieldIndex, err := getFieldIndex(chartStatFieldName, tableValues)
	if err != nil {
		slog.Error("no gaudi chartStatFieldName field found")
		return ""
	}
	// group the data points by module_id
	moduleStat := make(map[string][]float64)
	for i := range tableValues.Fields[0].Values {
		moduleId := tableValues.Fields[moduleIdFieldIdx].Values[i]
		val, err := strconv.ParseFloat(tableValues.Fields[chartStatFieldIndex].Values[i], 64)
		if err != nil {
			slog.Error("error parsing utilization", slog.String("error", err.Error()))
			return ""
		}
		if _, ok := moduleStat[moduleId]; !ok {
			moduleStat[moduleId] = []float64{}
		}
		moduleStat[moduleId] = append(moduleStat[moduleId], val)
	}
	// sort the module ids
	var moduleIds []string
	for moduleId := range moduleStat {
		moduleIds = append(moduleIds, moduleId)
	}
	sort.Strings(moduleIds)
	// build the data
	for _, moduleId := range moduleIds {
		if len(moduleStat[moduleId]) > 0 {
			data = append(data, moduleStat[moduleId])
			datasetNames = append(datasetNames, "module "+moduleId)
		}
	}
	chartConfig := chartTemplateStruct{
		ID:            fmt.Sprintf("%s%d", tableValues.Name, rand.IntN(10000)),
		XaxisText:     "Time",
		YaxisText:     yAxisText,
		TitleText:     titleText,
		DisplayTitle:  "true",
		DisplayLegend: "true",
		AspectRatio:   "2",
		SuggestedMin:  "0",
		SuggestedMax:  suggestedMax,
	}
	return telemetryTableHTMLRenderer(tableValues, data, datasetNames, chartConfig)
}

func gaudiStatsTableHTMLRenderer(tableValues TableValues, targetName string) string {
	out := ""
	out += renderGaudiStatsChart(tableValues, "utilization.aip [%]", "Utilization", "% Utilization", "100")
	out += renderGaudiStatsChart(tableValues, "memory.free [MiB]", "Memory Free", "Memory (MiB)", "0")
	out += renderGaudiStatsChart(tableValues, "memory.used [MiB]", "Memory Used", "Memory (MiB)", "0")
	out += renderGaudiStatsChart(tableValues, "power.draw [W]", "Power", "Watts", "0")
	out += renderGaudiStatsChart(tableValues, "temperature.aip [C]", "Temperature", "Temperature (C)", "0")
	return out
}

func codePathFrequencyTableHTMLRenderer(tableValues TableValues, targetName string) string {
	out := `<style>

/* Custom page header */
.fgheader {
	padding-bottom: 15px;
	padding-right: 15px;
	padding-left: 15px;
	border-bottom: 1px solid #e5e5e5;
}

/* Make the masthead heading the same height as the navigation */
.fgheader h3 {
    margin-top: 0;
    margin-bottom: 0;
    line-height: 40px;
}

/* Customize container */
.fgcontainer {
	max-width: 990px;
}
</style>
`
	out += renderFlameGraph("System", tableValues, "System Paths")
	out += renderFlameGraph("Java", tableValues, "Java Paths")
	return out
}

func kernelLockAnalysisHTMLRenderer(tableValues TableValues, targetName string) string {
	values := [][]string{}
	var tableValueStyles [][]string
	for _, field := range tableValues.Fields {
		rowValues := []string{}
		rowValues = append(rowValues, field.Name)
		rowValues = append(rowValues, htmltemplate.HTMLEscapeString(field.Values[0]))
		values = append(values, rowValues)
		rowStyles := []string{}
		rowStyles = append(rowStyles, "font-weight:bold")
		rowStyles = append(rowStyles, "white-space: pre-wrap")
		tableValueStyles = append(tableValueStyles, rowStyles)
	}
	return renderHTMLTable([]string{}, values, "pure-table pure-table-striped", tableValueStyles)
}
